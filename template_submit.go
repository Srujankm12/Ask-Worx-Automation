package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

// The panel's Templates page writes a marketing template and sends it to Meta
// for review, so ASKworX never has to open WhatsApp Manager. Only the shape
// the broadcaster can send is offered: an image header, fixed body text and
// up to three quick-reply buttons. templateProblem refuses anything else when
// a broadcast is saved, so a template made here is always usable once approved.

// Meta's limits for a template, checked here so the operator gets a plain
// answer instead of Meta's error codes.
const (
	templateBodyLimit   = 1024
	templateButtonLimit = 25
	templateImageLimit  = 5 << 20 // Meta refuses header images over 5 MB
)

var templateNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,512}$`)

type templateDraft struct {
	Name     string   `json:"name"`
	Body     string   `json:"body"`
	ImageURL string   `json:"image_url"`
	Buttons  []string `json:"buttons"`
}

// check tidies the draft and returns why Meta would refuse it, or "".
func (d *templateDraft) check() string {
	d.Name = strings.TrimSpace(d.Name)
	d.Body = strings.TrimSpace(d.Body)
	if !templateNamePattern.MatchString(d.Name) {
		return "the name can only use lowercase letters, numbers and _"
	}
	if d.Body == "" {
		return "write the message text"
	}
	if n := len([]rune(d.Body)); n > templateBodyLimit {
		return fmt.Sprintf("the message is %d characters; WhatsApp allows %d", n, templateBodyLimit)
	}
	if strings.Contains(d.Body, "{{") {
		return "the message cannot contain {{ }}; the bot has nothing to fill them with"
	}
	if strings.TrimSpace(d.ImageURL) == "" {
		return "choose a photo; the broadcast's own poster replaces it when it is sent"
	}
	if len(d.Buttons) > 3 {
		return "a template can have at most 3 buttons"
	}
	seen := map[string]bool{}
	for i := range d.Buttons {
		d.Buttons[i] = strings.TrimSpace(d.Buttons[i])
		b := d.Buttons[i]
		if b == "" {
			return "every button needs text"
		}
		if len([]rune(b)) > templateButtonLimit {
			return fmt.Sprintf("button text %q is longer than %d characters", b, templateButtonLimit)
		}
		if seen[b] {
			return "two buttons cannot have the same text"
		}
		seen[b] = true
	}
	return ""
}

// metaError pulls the explanation out of a Graph API error. Meta's message
// for the person using the app is clearer than its developer message.
func metaError(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
			UserMsg string `json:"error_user_msg"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil {
		return "Meta did not say why"
	}
	if e.Error.UserMsg != "" {
		return e.Error.UserMsg
	}
	if e.Error.Message != "" {
		return e.Error.Message
	}
	return "Meta did not say why"
}

// graphCall sends one request to the Graph API and decodes a success into out.
func graphCall(method, endpoint string, body io.Reader, header http.Header, out interface{}) error {
	req, err := http.NewRequest(method, "https://graph.facebook.com/v17.0/"+endpoint, body)
	if err != nil {
		return err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))
	}
	resp, err := metaClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Meta: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("❌ [Templates] %s %s: Meta answered %d: %s", method, path.Base(endpoint), resp.StatusCode, data)
		return fmt.Errorf("%s", metaError(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// The upload that gives Meta a sample header image belongs to the app, which
// the access token already identifies, so no extra setting is needed.
var metaApp struct {
	sync.Mutex
	id string
}

func metaAppID() (string, error) {
	metaApp.Lock()
	defer metaApp.Unlock()
	if metaApp.id != "" {
		return metaApp.id, nil
	}
	var app struct {
		ID string `json:"id"`
	}
	if err := graphCall("GET", "app?fields=id", nil, nil, &app); err != nil {
		return "", fmt.Errorf("could not find the Meta app for the access token: %w", err)
	}
	metaApp.id = app.ID
	return app.ID, nil
}

// uploadSampleImage gives Meta the template's example header image and
// returns the handle a template submission refers to it by.
func uploadSampleImage(imageURL string) (string, error) {
	i := strings.Index(imageURL, "/uploads/")
	if i < 0 {
		return "", fmt.Errorf("upload the photo from this panel")
	}
	name := path.Base(imageURL[i:])
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	data, err := os.ReadFile(filepath.Join("./uploads", name))
	if err != nil {
		return "", fmt.Errorf("that photo is no longer on the server; upload it again")
	}
	if len(data) > templateImageLimit {
		return "", fmt.Errorf("the photo is larger than 5 MB; use a smaller one")
	}
	kind := http.DetectContentType(data)
	if kind != "image/jpeg" && kind != "image/png" {
		return "", fmt.Errorf("WhatsApp takes JPG or PNG photos for templates")
	}

	app, err := metaAppID()
	if err != nil {
		return "", err
	}
	var session struct {
		ID string `json:"id"`
	}
	q := url.Values{
		"file_name":   {name},
		"file_length": {fmt.Sprint(len(data))},
		"file_type":   {kind},
	}
	if err := graphCall("POST", app+"/uploads?"+q.Encode(), nil, nil, &session); err != nil {
		return "", fmt.Errorf("Meta did not accept the photo: %w", err)
	}
	var done struct {
		Handle string `json:"h"`
	}
	header := http.Header{
		"Authorization": {"OAuth " + os.Getenv("ACCESS_TOKEN")},
		"File_offset":   {"0"},
	}
	if err := graphCall("POST", session.ID, bytes.NewReader(data), header, &done); err != nil {
		return "", fmt.Errorf("Meta did not accept the photo: %w", err)
	}
	return done.Handle, nil
}

// forgetTemplates drops the cached list, so the panel sees a change at once.
func forgetTemplates() {
	templateCache.Lock()
	templateCache.list = nil
	templateCache.Unlock()
}

// createTemplateHandler sends a new marketing template to Meta for review.
func createTemplateHandler(w http.ResponseWriter, r *http.Request) {
	var d templateDraft
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&d); err != nil {
		writeJSONError(w, http.StatusBadRequest, "That template could not be read.")
		return
	}
	if problem := d.check(); problem != "" {
		writeJSONError(w, http.StatusBadRequest, "Not sent: "+problem+".")
		return
	}
	waba := strings.TrimSpace(os.Getenv("WABA_ID"))
	if waba == "" {
		writeJSONError(w, http.StatusServiceUnavailable, "WABA_ID is not set on the server, so templates cannot be sent to Meta.")
		return
	}

	handle, err := uploadSampleImage(d.ImageURL)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "Not sent: "+err.Error())
		return
	}

	components := []map[string]interface{}{
		{"type": "HEADER", "format": "IMAGE", "example": map[string][]string{"header_handle": {handle}}},
		{"type": "BODY", "text": d.Body},
	}
	if len(d.Buttons) > 0 {
		buttons := make([]map[string]string, 0, len(d.Buttons))
		for _, b := range d.Buttons {
			buttons = append(buttons, map[string]string{"type": "QUICK_REPLY", "text": b})
		}
		components = append(components, map[string]interface{}{"type": "BUTTONS", "buttons": buttons})
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"name":       d.Name,
		"language":   "en",
		"category":   "MARKETING",
		"components": components,
	})

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	header := http.Header{"Content-Type": {"application/json"}}
	if err := graphCall("POST", waba+"/message_templates", bytes.NewReader(payload), header, &created); err != nil {
		writeJSONError(w, http.StatusBadGateway, "Meta did not accept the template: "+err.Error())
		return
	}
	forgetTemplates()
	log.Printf("[Templates] Sent %q to Meta for review (id %s, %s)", d.Name, created.ID, created.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(created)
}

// deleteTemplateHandler removes a template from the WhatsApp account. Meta
// keeps the name reserved for a while after, so it cannot be reused at once.
func deleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !templateNamePattern.MatchString(name) {
		writeJSONError(w, http.StatusBadRequest, "That is not a template name.")
		return
	}
	waba := strings.TrimSpace(os.Getenv("WABA_ID"))
	if waba == "" {
		writeJSONError(w, http.StatusServiceUnavailable, "WABA_ID is not set on the server.")
		return
	}
	if err := graphCall("DELETE", waba+"/message_templates?name="+url.QueryEscape(name), nil, nil, nil); err != nil {
		writeJSONError(w, http.StatusBadGateway, "Meta did not delete the template: "+err.Error())
		return
	}
	forgetTemplates()
	log.Printf("[Templates] Deleted %q", name)
	w.WriteHeader(http.StatusNoContent)
}
