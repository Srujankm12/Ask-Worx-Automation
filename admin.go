package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"askworx-whatsapp-bot/db"

	"github.com/go-chi/chi/v5"
)

// Maximum size for a broadcast poster image stored in the database, and the
// only formats accepted. Enforced here — never just on the frontend, since a
// request can always skip the browser entirely.
const maxCampaignImageBytes = 2 << 20 // 2 MB

var allowedCampaignImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// parseCampaignMultipart reads a multipart POST /campaigns request into c and
// returns the uploaded image's bytes, if any. On an invalid request it writes
// the error response itself and returns ok=false.
func parseCampaignMultipart(w http.ResponseWriter, r *http.Request, c *db.Campaign) (imageData []byte, ok bool) {
	// A little over the image cap to leave room for the rest of the form; the
	// image itself is still checked against the real 2 MB limit below.
	const maxRequest = maxCampaignImageBytes + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxRequest)
	if err := r.ParseMultipartForm(maxRequest); err != nil {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "Image size must be less than or equal to 2 MB.")
		return nil, false
	}

	c.Type = r.FormValue("type")
	c.Question = r.FormValue("question")
	c.OptionA = r.FormValue("option_a")
	c.OptionB = r.FormValue("option_b")
	c.OptionC = r.FormValue("option_c")
	c.CorrectAnswer = r.FormValue("correct_answer")
	c.Explanation = r.FormValue("explanation")
	c.YouTubeLink = r.FormValue("youtube_link")
	c.ImageURL = r.FormValue("image_url")
	c.Caption = r.FormValue("caption")
	c.Title = r.FormValue("title")
	c.Description = r.FormValue("description")
	c.Links = r.FormValue("links")

	if scheduledAt := r.FormValue("scheduled_at"); scheduledAt != "" {
		t, err := time.Parse(time.RFC3339, scheduledAt)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "scheduled_at must be a valid date and time.")
			return nil, false
		}
		c.ScheduledAt = t
	}

	file, handler, err := r.FormFile("image")
	if err == http.ErrMissingFile {
		return nil, true
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Could not read the uploaded image.")
		return nil, false
	}
	defer file.Close()

	if handler.Size > maxCampaignImageBytes {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "Image size must be less than or equal to 2 MB.")
		return nil, false
	}

	// The browser-supplied filename and Content-Type are never trusted — only
	// what the bytes themselves sniff as. A renamed .exe would otherwise pass
	// a naive extension check.
	head := make([]byte, 512)
	n, _ := file.Read(head)
	sniffed := http.DetectContentType(head[:n])
	if !allowedCampaignImageTypes[sniffed] {
		writeJSONError(w, http.StatusUnsupportedMediaType, "Only JPG, JPEG, PNG and WEBP images are allowed.")
		return nil, false
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Could not read that image.")
		return nil, false
	}

	// Read one byte past the cap so an oversized file is rejected outright
	// instead of silently truncated to 2 MB.
	data, err := io.ReadAll(io.LimitReader(file, maxCampaignImageBytes+1))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Could not read that image.")
		return nil, false
	}
	if len(data) > maxCampaignImageBytes {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "Image size must be less than or equal to 2 MB.")
		return nil, false
	}
	if len(data) == 0 {
		writeJSONError(w, http.StatusBadRequest, "That image file is empty.")
		return nil, false
	}

	c.ImageName = strings.ReplaceAll(filepath.Base(handler.Filename), " ", "_")
	c.ImageType = sniffed
	c.ImageSize = int64(len(data))
	return data, true
}

func AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Use(AuthMiddleware)

	// ── STATIC FILE SERVING FOR UPLOADS ──────────────────────────────────────
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0755)
	}
	// Served publicly so Meta can fetch poster images. X-Content-Type-Options
	// stops a browser sniffing one of these into something executable, and the
	// CSP is a second line behind the image-only check on the upload itself.
	r.Handle("/uploads/*", http.StripPrefix("/uploads/",
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'")
				next.ServeHTTP(w, r)
			})
		}(http.FileServer(http.Dir(uploadDir)))))

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		// Cap the whole request, not just what is buffered in memory.
		// ParseMultipartForm alone spills the remainder to disk, so a large
		// upload could fill the volume.
		const maxUpload = 8 << 20 // 8 MiB
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			http.Error(w, "That file is too large. The limit is 8 MB.", http.StatusRequestEntityTooLarge)
			return
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "error retrieving file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Only images. These files are served back from this origin, so
		// accepting arbitrary types would let an uploaded .html or .svg run
		// script against the API's own origin.
		head := make([]byte, 512)
		n, _ := file.Read(head)
		contentType := http.DetectContentType(head[:n])
		if !strings.HasPrefix(contentType, "image/") {
			http.Error(w, "Only image files can be uploaded.", http.StatusUnsupportedMediaType)
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "Could not read that file.", http.StatusInternalServerError)
			return
		}

		safeFilename := strings.ReplaceAll(handler.Filename, " ", "_")
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(safeFilename))
		dst, err := os.Create(filepath.Join(uploadDir, filename))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": "/uploads/" + filename})
	})

	r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
		contacts, err1 := db.GetAllContacts()
		leads, err2 := db.GetAllLeads()
		callbacks, err3 := db.GetAllCallbacks()
		messages, err4 := db.GetAllMessages()

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			fmt.Printf("Error fetching stats: %v %v %v %v\n", err1, err2, err3, err4)
		}

		stats := map[string]int{
			"total_contacts":    len(contacts),
			"total_leads":       len(leads),
			"pending_callbacks": 0,
			"new_leads":         0,
			"total_messages":    len(messages),
		}

		for _, l := range leads {
			if l.Status == "new" {
				stats["new_leads"]++
			}
		}
		for _, c := range callbacks {
			if c.Status == "pending" {
				stats["pending_callbacks"]++
			}
		}

		json.NewEncoder(w).Encode(stats)
	})

	r.Get("/leads", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, start, end := parseCommonParams(r)
		leads, err := db.GetLeadsPaginated(limit, offset, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalLeadsCount(start, end)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": leads, "total": count})
	})

	r.Post("/leads/update-status", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		db.UpdateLeadStatus(body.ID, body.Status)
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/callbacks", func(w http.ResponseWriter, r *http.Request) {
		callbacks, _ := db.GetAllCallbacks()
		json.NewEncoder(w).Encode(callbacks)
	})

	r.Post("/callbacks/mark-done", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID int `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		db.MarkCallbackDone(body.ID)
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/contacts", func(w http.ResponseWriter, r *http.Request) {
		contacts, _ := db.GetAllContacts()
		json.NewEncoder(w).Encode(contacts)
	})

	r.Post("/contacts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone   string `json:"phone"`
			Name    string `json:"name"`
			Company string `json:"company"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.UpsertContact(body.Phone, body.Name, body.Company); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/contacts/import", ImportContactsHandler)

	r.Delete("/contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if err := db.DeleteContact(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/contacts/{id}/opt-out", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)

		var body struct {
			OptOut bool `json:"opt_out"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := db.UpdateContactOptOut(id, body.OptOut); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Bounded. This used to call GetAllMessages, which has no LIMIT — the
	// handler returned every row ever written and ignored the limit the client
	// sent, so the payload grew without bound for the life of the deployment.
	r.Get("/messages", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		messages, total, err := db.GetMessagesPage(limit, offset)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not read the message log.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": messages, "total": total})
	})

	// Per-day and per-hour counts for the dashboard charts, aggregated in the
	// database rather than by shipping the log to the browser.
	r.Get("/messages/summary", func(w http.ResponseWriter, r *http.Request) {
		days, _ := strconv.Atoi(r.URL.Query().Get("days"))
		summary, err := db.GetMessageSummary(days)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not read the conversation summary.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summary)
	})

	// ?after_id=N returns only what is newer, so the inbox poll does not refetch
	// the whole conversation every few seconds.
	r.Get("/messages/{phone}", func(w http.ResponseWriter, r *http.Request) {
		phone := chi.URLParam(r, "phone")
		afterID, _ := strconv.Atoi(r.URL.Query().Get("after_id"))

		messages, err := db.GetMessagesByPhoneAfter(phone, afterID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not read that conversation.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	r.Post("/send-message", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone   string `json:"phone"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
			strings.TrimSpace(body.Phone) == "" || strings.TrimSpace(body.Message) == "" {
			writeJSONError(w, http.StatusBadRequest, "A phone number and a message are both needed.")
			return
		}

		// Outside the 24-hour window Meta rejects a free-form message, and
		// sendTextMessage only logs that. Refusing here means the panel says
		// it was not sent instead of showing a reply the customer never got.
		inWindow, err := db.IsInServiceWindow(body.Phone)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not check whether this contact can be messaged.")
			return
		}
		if !inWindow {
			writeJSONError(w, http.StatusConflict,
				"This contact has not messaged in the last 24 hours, so WhatsApp will not deliver a free reply. They need to message first.")
			return
		}

		sendTextMessage(body.Phone, body.Message)
		w.WriteHeader(http.StatusOK)
	})

	// ── Campaign Management ─────────────────────────────────────────────────

	r.Get("/campaigns", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, start, end := parseCommonParams(r)
		campaigns, err := db.GetCampaignsPaginated(limit, offset, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalCampaignsCount(start, end)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": campaigns, "total": count})
	})

	r.Post("/campaigns", func(w http.ResponseWriter, r *http.Request) {
		var c db.Campaign
		var imageData []byte

		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			data, ok := parseCampaignMultipart(w, r, &c)
			if !ok {
				return
			}
			imageData = data
		} else if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validation. A broadcast is a poster: an image, a title and a
		// description. Quizzes are no longer sent, so "poster" is the only
		// type this accepts.
		if c.Type != "poster" {
			writeJSONError(w, http.StatusBadRequest, "type must be 'poster'")
			return
		}
		if c.ImageURL == "" && len(imageData) == 0 {
			writeJSONError(w, http.StatusBadRequest, "image_url or an uploaded image is required")
			return
		}
		if strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Description) == "" {
			writeJSONError(w, http.StatusBadRequest, "title and description are required")
			return
		}
		if c.ScheduledAt.IsZero() {
			writeJSONError(w, http.StatusBadRequest, "scheduled_at is required")
			return
		}

		id, err := db.CreateCampaign(c, imageData)
		if err != nil {
			log.Printf("[Campaigns] could not create campaign: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Could not save that broadcast. Please try again.")
			return
		}

		// The image lives in the database, not at c.ImageURL — point the row at
		// the endpoint that serves it back, now that its id exists, so the
		// scheduler and the panel can both load it exactly like a pasted link.
		if len(imageData) > 0 {
			publicURL := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")
			imageURL := fmt.Sprintf("%s/api/campaigns/%d/image", publicURL, id)
			if err := db.SetCampaignImageURL(id, imageURL); err != nil {
				log.Printf("[Campaigns] could not set image_url for campaign %d: %v", id, err)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	})

	r.Delete("/campaigns/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if err := db.CancelCampaign(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Serves a poster's image straight out of the database. Public (see
	// AuthMiddleware) so both the panel's <img> tags and Meta's own fetch of
	// the WhatsApp message can load it without a session token.
	r.Get("/campaigns/{id}/image", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)

		data, contentType, err := db.GetCampaignImage(id)
		if err != nil || len(data) == 0 {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		w.Write(data)
	})

	// ── Employee Management ─────────────────────────────────────────────────

	// ── Saved broadcast templates ───────────────────────────────────────────

	r.Get("/templates", func(w http.ResponseWriter, r *http.Request) {
		templates, err := db.GetTemplates()
		if err != nil {
			log.Printf("[Templates] could not list: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "Could not load your saved templates.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(templates)
	})

	r.Post("/templates", func(w http.ResponseWriter, r *http.Request) {
		var t db.Template
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		t.Name = strings.TrimSpace(t.Name)
		if t.Name == "" {
			writeJSONError(w, http.StatusBadRequest, "Give the template a name.")
			return
		}
		if strings.TrimSpace(t.Title) == "" && strings.TrimSpace(t.Description) == "" {
			writeJSONError(w, http.StatusBadRequest, "A template needs a title or a description.")
			return
		}
		id, err := db.CreateTemplate(t)
		if err != nil {
			log.Printf("[Templates] could not save %q: %v", t.Name, err)
			writeJSONError(w, http.StatusInternalServerError, "Could not save that template. Please try again.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	})

	r.Delete("/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(chi.URLParam(r, "id"), "%d", &id)
		if err := db.DeleteTemplate(id); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not delete that template.")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/employees", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, _, _ := parseCommonParams(r)
		employees, err := db.GetEmployeesPaginated(limit, offset)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalEmployeesCount()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": employees, "total": count})
	})

	r.Post("/employees", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if err := db.AddEmployee(body.Name, body.Phone); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Delete("/employees/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		db.DeleteEmployee(id)
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/attendance", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, start, end := parseCommonParams(r)
		records, err := db.GetAttendancePaginated(limit, offset, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalAttendanceCount(start, end)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": records, "total": count})
	})

	r.Get("/leave-requests", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, start, end := parseCommonParams(r)
		requests, err := db.GetLeaveRequestsPaginated(limit, offset, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalLeaveRequestsCount(start, end)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": requests, "total": count})
	})

	r.Post("/leave-requests/update-status", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		phone, err := db.UpdateLeaveStatus(body.ID, body.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Notify employee via WhatsApp
		// {{status}} is specific to this template. The employee is told the
		// outcome in a word, not left to infer it from a tone.
		msg := strings.ReplaceAll(
			renderTemplate(db.SettingOr("emp_leave_decision",
				"*Leave request {{status}}*\n\nYour leave request has been {{status}}.\n\n— {{company}}"),
				db.GetEmployeeName(phone)),
			"{{status}}", body.Status)
		sendFAQAnswer(phone, msg) // Use the one with Main Menu button

		w.WriteHeader(http.StatusOK)
	})

	r.Get("/reminders/history", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, start, end := parseCommonParams(r)
		reminders, err := db.GetReminders(limit, offset, start, end)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		count, _ := db.GetTotalRemindersCount(start, end)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": reminders, "total": count})
	})

	r.Post("/reminders", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone string    `json:"phone"`
			Desc  string    `json:"desc"`
			Due   time.Time `json:"due"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Printf("[Reminders] Failed to decode body: %v", err)
			http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Phone == "" || body.Desc == "" || body.Due.IsZero() {
			log.Printf("[Reminders] Missing fields: phone=%s desc=%s due=%v", body.Phone, body.Desc, body.Due)
			http.Error(w, "phone, desc, and due are all required", http.StatusBadRequest)
			return
		}
		log.Printf("[Reminders] Creating reminder for %s at %v: %s", body.Phone, body.Due, body.Desc)
		if err := db.CreateReminder(body.Phone, body.Desc, body.Due); err != nil {
			log.Printf("[Reminders] DB error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/announcements", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message string   `json:"message"`
			Phones  []string `json:"phones"` // Empty means all
		}
		json.NewDecoder(r.Body).Decode(&body)

		targets := body.Phones
		if len(targets) == 0 {
			emps, _ := db.GetAllEmployees()
			for _, e := range emps {
				targets = append(targets, e.Phone)
			}
		}

		// Store each broadcast in reminders table as 'sent' for history tracking
		now := time.Now()
		for _, p := range targets {
			db.CreateAnnouncementRecord(p, body.Message, now)
		}

		go func() {
			// The panel supplies {{message}}; this template is everything
			// wrapped around it. Previously the wrapper was fixed, so an
			// administrator wrote the middle of a message they could not see.
			fullMsg := strings.ReplaceAll(
				renderTemplate(db.SettingOr("emp_announcement",
					"*Announcement*\n\n{{message}}\n\n— {{company}}"), ""),
				"{{message}}", body.Message)
			for _, p := range targets {
				sendFAQAnswer(p, fullMsg)
				time.Sleep(500 * time.Millisecond) // Rate limit
			}
		}()

		log.Printf("[Announcements] Broadcast queued to %d recipients", len(targets))
		w.WriteHeader(http.StatusOK)
	})

	// ── BOT SETTINGS ─────────────────────────────────────────────────────────
	r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
		settings, err := db.GetAllSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	})

	r.Post("/settings", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		for k, v := range payload {
			db.UpdateSetting(k, v)
		}
		w.WriteHeader(http.StatusOK)
	})

	// ── FAQ / KNOWLEDGE BASE ──────────────────────────────────────────────
	r.Get("/faqs", func(w http.ResponseWriter, r *http.Request) {
		faqs, err := db.GetAllFAQs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(faqs)
	})

	r.Post("/faqs", func(w http.ResponseWriter, r *http.Request) {
		var f db.FAQ
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.SaveFAQ(f); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Delete("/faqs/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		if err := db.DeleteFAQ(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return r
}

func parseCommonParams(r *http.Request) (limit, offset int, start, end string) {
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
	if limit <= 0 {
		limit = 10
	}
	start = r.URL.Query().Get("start_date")
	end = r.URL.Query().Get("end_date")
	return
}
