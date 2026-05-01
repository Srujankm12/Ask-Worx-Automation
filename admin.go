package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"askworx-whatsapp-bot/db"

	"github.com/go-chi/chi/v5"
)

func AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Use(AuthMiddleware)


	// ── STATIC FILE SERVING FOR UPLOADS ──────────────────────────────────────
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0755)
	}
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	r.Post("/upload", func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20) // 10MB max
		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "error retrieving file", http.StatusBadRequest)
			return
		}
		defer file.Close()

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
			Phone string `json:"phone"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.UpsertContact(body.Phone, body.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

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

	r.Get("/messages", func(w http.ResponseWriter, r *http.Request) {
		messages, _ := db.GetAllMessages()
		json.NewEncoder(w).Encode(messages)
	})

	r.Get("/messages/{phone}", func(w http.ResponseWriter, r *http.Request) {
		phone := chi.URLParam(r, "phone")
		messages, _ := db.GetMessagesByPhone(phone)
		json.NewEncoder(w).Encode(messages)
	})

	r.Post("/send-message", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phone   string `json:"phone"`
			Message string `json:"message"`
		}
		json.NewDecoder(r.Body).Decode(&body)
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
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// Validation
		if c.Type != "quiz" && c.Type != "poster" {
			http.Error(w, "type must be 'quiz' or 'poster'", http.StatusBadRequest)
			return
		}
		if c.Type == "quiz" {
			if c.Question == "" || c.OptionA == "" || c.OptionB == "" || c.OptionC == "" || c.Explanation == "" {
				http.Error(w, "all quiz fields are required", http.StatusBadRequest)
				return
			}
			c.CorrectAnswer = strings.ToUpper(c.CorrectAnswer)
			if c.CorrectAnswer != "A" && c.CorrectAnswer != "B" && c.CorrectAnswer != "C" {
				http.Error(w, "correct_answer must be A, B, or C", http.StatusBadRequest)
				return
			}
			if len([]rune(c.Explanation)) > 300 {
				http.Error(w, "explanation must not exceed 300 characters", http.StatusBadRequest)
				return
			}
		}
		if c.Type == "poster" && c.ImageURL == "" {
			http.Error(w, "image_url is required for posters", http.StatusBadRequest)
			return
		}
		if c.ScheduledAt.IsZero() {
			http.Error(w, "scheduled_at is required", http.StatusBadRequest)
			return
		}

		id, err := db.CreateCampaign(c)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

	r.Get("/campaigns/{id}/analytics", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		var id int
		fmt.Sscanf(idStr, "%d", &id)
		analytics, err := db.GetCampaignAnalytics(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analytics)
	})

	// ── Employee Management ─────────────────────────────────────────────────

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
		msg := fmt.Sprintf("📢 *Leave Request Update*\n\nYour leave request has been *%s* by the management.", body.Status)
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
			fullMsg := "📢 *O F F I C I A L   B R O A D C A S T* 📢\n\n" +
				"Team " + os.Getenv("COMPANY_NAME") + ",\n\n" +
				"*" + body.Message + "*\n\n" +
				"United by innovation, driven by excellence. Let's keep pioneering the future of industry, one milestone at a time. 🌍✨\n\n" +
				"Regards,\n*" + os.Getenv("COMPANY_NAME") + " Team*"
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

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Password == os.Getenv("ADMIN_PASSWORD") {
		secret := os.Getenv("API_SECRET")
		if secret == "" {
			secret = "dummy-token-askworx"
		}
		json.NewEncoder(w).Encode(map[string]string{"token": secret})
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login and public uploads
		// Note: chi might have different paths depending on where it's mounted
		path := r.URL.Path
		if path == "/api/login" || path == "/login" || 
		   strings.HasPrefix(path, "/uploads") || 
		   strings.HasPrefix(path, "/api/uploads") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		secret := os.Getenv("API_SECRET")
		if secret == "" {
			secret = "dummy-token-askworx"
		}

		if token != secret {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
