package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// WhatsApp only delivers a free-form message to someone who wrote in the last
// 24 hours. Everyone else can be reached only with a template Meta approved
// in advance, and Meta charges for each one delivered. A broadcast can name a
// marketing template; the broadcaster sends it to subscribed contacts outside
// the window, with the poster image as the template's image header.

// MetaTemplate is a message template as Meta's API describes it.
type MetaTemplate struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Language       string              `json:"language"`
	Category       string              `json:"category"`
	Status         string              `json:"status"`
	RejectedReason string              `json:"rejected_reason"`
	Components     []TemplateComponent `json:"components"`
}

type TemplateComponent struct {
	Type    string           `json:"type"`   // HEADER | BODY | FOOTER | BUTTONS
	Format  string           `json:"format"` // header only: TEXT | IMAGE | VIDEO | DOCUMENT | LOCATION
	Text    string           `json:"text"`
	Buttons []TemplateButton `json:"buttons"`
	// The sample Meta's reviewers saw; for an image header, a link to it.
	Example *struct {
		HeaderHandle []string `json:"header_handle"`
	} `json:"example,omitempty"`
}

type TemplateButton struct {
	Type string `json:"type"` // QUICK_REPLY | URL | PHONE_NUMBER | ...
	Text string `json:"text"`
	URL  string `json:"url"`
}

// templateProblem returns why the broadcaster cannot send t, or "" if it can.
// It fills in only the image header and quick-reply payloads, so a template
// that needs text filled in ({{1}}) or another kind of header is refused when
// the broadcast is saved rather than failing for every contact at send time.
func templateProblem(t MetaTemplate) string {
	if t.Status != "APPROVED" {
		return "it is not approved by Meta yet"
	}
	if t.Category != "MARKETING" {
		return "it is not a marketing template"
	}
	for _, c := range t.Components {
		switch c.Type {
		case "HEADER":
			if c.Format != "IMAGE" && c.Format != "TEXT" {
				return "its header must be an image or plain text"
			}
		case "BUTTONS":
			for _, b := range c.Buttons {
				if b.Type == "URL" && strings.Contains(b.URL, "{{") {
					return "its link button needs a value filled in"
				}
			}
		}
		if strings.Contains(c.Text, "{{") {
			return "its text has {{ }} values to fill in; use one without them"
		}
	}
	return ""
}

func (t MetaTemplate) component(kind string) (TemplateComponent, bool) {
	for _, c := range t.Components {
		if c.Type == kind {
			return c, true
		}
	}
	return TemplateComponent{}, false
}

// The list barely changes, and the composer reads it each time it opens.
var templateCache struct {
	sync.Mutex
	list    []MetaTemplate
	fetched time.Time
}

const templateCacheFor = 2 * time.Minute

// fetchTemplates returns the business account's marketing templates. It
// needs WABA_ID: templates belong to the WhatsApp Business Account, not to
// the phone number.
func fetchTemplates() ([]MetaTemplate, error) {
	templateCache.Lock()
	defer templateCache.Unlock()
	if templateCache.list != nil && time.Since(templateCache.fetched) < templateCacheFor {
		return templateCache.list, nil
	}

	waba := strings.TrimSpace(os.Getenv("WABA_ID"))
	if waba == "" {
		return nil, fmt.Errorf("WABA_ID is not set, so the bot cannot read your WhatsApp templates")
	}
	q := url.Values{
		"fields":   {"id,name,language,category,status,rejected_reason,components"},
		"category": {"MARKETING"},
		"limit":    {"100"},
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://graph.facebook.com/v17.0/%s/message_templates?%s", waba, q.Encode()), nil)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("ACCESS_TOKEN"))
	resp, err := metaClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Meta: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("❌ [Templates] Meta answered %d: %s", resp.StatusCode, body)
		return nil, fmt.Errorf("Meta refused the template list (status %d)", resp.StatusCode)
	}

	var page struct {
		Data []MetaTemplate `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("could not read Meta's template list: %w", err)
	}
	templateCache.list, templateCache.fetched = page.Data, time.Now()
	return page.Data, nil
}

// findTemplate looks up one template by name and language.
func findTemplate(name, lang string) (MetaTemplate, error) {
	list, err := fetchTemplates()
	if err != nil {
		return MetaTemplate{}, err
	}
	for _, t := range list {
		if t.Name == name && t.Language == lang {
			return t, nil
		}
	}
	return MetaTemplate{}, fmt.Errorf("there is no template %q (%s) on your WhatsApp account", name, lang)
}

// sendTemplate sends t to one contact and reports whether Meta accepted it.
// An image header carries the poster image. Quick-reply buttons are given
// actions in order, so a tap does what the same button does on the free
// message.
func sendTemplate(to string, t MetaTemplate, imageURL string, actions []string) metaResult {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "template",
		"template":          templateMessage(t, imageURL, actions),
	}
	body := ""
	if c, ok := t.component("BODY"); ok {
		body = c.Text
	}
	return sendToMetaResult(payload, to, "[Template: "+t.Name+"] "+body)
}

// quickReplyActions gives each of t's quick-reply buttons the action a tap
// should take: a button that says to stop messages opts the person out, and
// any other does what the poster's button does. Without this only the first
// button had an action, so tapping "Stop messages" on a template came back as
// those words and opted nobody out.
func quickReplyActions(t MetaTemplate) []string {
	var actions []string
	if b, ok := t.component("BUTTONS"); ok {
		for _, btn := range b.Buttons {
			if btn.Type != "QUICK_REPLY" {
				continue
			}
			if isOptOut(strings.ToLower(btn.Text)) {
				actions = append(actions, "opt_out")
			} else {
				actions = append(actions, posterButtonAction)
			}
		}
	}
	return actions
}

// sampleImage is the picture Meta's reviewers saw in t's image header, which
// a send to one contact uses since there is no poster to take one from.
func sampleImage(t MetaTemplate) string {
	if h, ok := t.component("HEADER"); ok && h.Example != nil && len(h.Example.HeaderHandle) > 0 {
		return h.Example.HeaderHandle[0]
	}
	return ""
}

// templateMessage is the "template" object of a send: the template's name and
// language, and the values its image header and quick-reply buttons take.
func templateMessage(t MetaTemplate, imageURL string, actions []string) map[string]interface{} {
	var components []map[string]interface{}
	if h, ok := t.component("HEADER"); ok && h.Format == "IMAGE" {
		components = append(components, map[string]interface{}{
			"type": "header",
			"parameters": []map[string]interface{}{
				{"type": "image", "image": map[string]string{"link": imageURL}},
			},
		})
	}
	if b, ok := t.component("BUTTONS"); ok {
		quick := 0
		for i, btn := range b.Buttons {
			if btn.Type != "QUICK_REPLY" {
				continue
			}
			if quick < len(actions) {
				components = append(components, map[string]interface{}{
					"type":     "button",
					"sub_type": "quick_reply",
					"index":    fmt.Sprint(i),
					"parameters": []map[string]string{
						{"type": "payload", "payload": actions[quick]},
					},
				})
			}
			quick++
		}
	}

	template := map[string]interface{}{
		"name":     t.Name,
		"language": map[string]string{"code": t.Language},
	}
	if len(components) > 0 {
		template["components"] = components
	}
	return template
}

// templatesHandler lists the account's marketing templates for the composer,
// each with whether the broadcaster can send it.
func templatesHandler(w http.ResponseWriter, r *http.Request) {
	type item struct {
		MetaTemplate
		Problem string `json:"problem"`
	}
	w.Header().Set("Content-Type", "application/json")

	list, err := fetchTemplates()
	if err != nil {
		// 200 with the reason: the composer shows it instead of the picker.
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []item{}, "error": err.Error()})
		return
	}
	out := make([]item, 0, len(list))
	for _, t := range list {
		out = append(out, item{MetaTemplate: t, Problem: templateProblem(t)})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"data": out})
}
