package main

import (
	"askworx-whatsapp-bot/db"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func InitScheduler() {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		log.Println("Error loading location Asia/Kolkata:", err)
		loc = time.UTC // Fallback
	}

	c := cron.New(cron.WithLocation(loc))

	// Morning Check-in for Internal Team (Nudge at 9 AM IST)
	_, err = c.AddFunc("0 9 * * *", func() {
		emps, err := db.GetAllEmployees()
		if err != nil {
			return
		}
		for _, e := range emps {
			template := db.GetSetting("greeting_employee")
			if template == "" {
				template = fmt.Sprintf("🌅 *Good Morning, {{name}}!* 🏆\n\nAnother day to pioneer industrial excellence. Don't forget to **Start Your Day** in the Internal Hub to log your focus objectives.\n\nLet's make an impact! 🚀", os.Getenv("COMPANY_NAME"))
			}
			msg := strings.ReplaceAll(template, "{{name}}", e.Name)
			sendEmployeeDashboard(e.Phone) // This will trigger the dashboard buttons
			sendTextMessage(e.Phone, msg)
		}
	})

	// Good Morning greeting for Users/Customers (Nudge at 9:30 AM IST)
	_, err = c.AddFunc("30 9 * * *", func() {
		phones, err := db.GetAllPhoneNumbers()
		if err != nil {
			return
		}
		for _, p := range phones {
			// Skip if they are an employee
			isEmp, _ := db.IsEmployee(p)
			if isEmp {
				continue
			}

			msg := db.GetSetting("greeting_customer")
			if msg == "" {
				msg = fmt.Sprintf("🌅 *Good Morning from %s!* 🏭\n\nWe hope you have a productive day ahead. If you need any assistance with Industrial Automation, IIoT, or Software solutions, we are just a message away. 🚀", os.Getenv("COMPANY_NAME"))
			}
			// Clean up legacy "Type MENU" text if it exists in DB setting
			msg = strings.ReplaceAll(msg, "\n\nType *MENU* anytime to explore our solutions!", "")
			msg = strings.ReplaceAll(msg, "Type *MENU* anytime to explore our solutions!", "")

			buttons := []Button{
				{ID: "main_menu", Title: "🏠 Explore Menu"},
				{ID: "opt_out", Title: "🛑 Stop Messages"},
			}
			sendInteractiveButtons(p, msg, buttons)
		}
	})

	// Daily check for new leads older than 24h (IST 10 AM)
	_, err = c.AddFunc("0 10 * * *", func() {
		log.Println("Starting daily lead follow-up check...")
		count, err := db.GetNewLeadsCount()
		if err != nil {
			log.Println("Error checking lead count:", err)
			return
		}

		if count > 0 {
			adminPhone := os.Getenv("ADMIN_PHONE")
			msg := fmt.Sprintf("⚠️ %s Alert!\nYou have [%d] new leads pending follow-up.\nLogin to dashboard to view details.", os.Getenv("COMPANY_NAME"), count)
			sendTextMessage(adminPhone, msg)
		}
	})
	if err != nil {
		log.Println("Error scheduling lead check:", err)
	}

	// ── Every minute: check for due reminders and notify ─────────────────────
	_, err = c.AddFunc("* * * * *", func() {
		due, err := db.GetDueReminders()
		if err != nil {
			log.Println("[Scheduler] Error fetching due reminders:", err)
			return
		}
		for _, r := range due {
			log.Printf("[Scheduler] Sending reminder #%d to %s", r.ID, r.Phone)
			msg := "⚡ *A C T I O N   R E Q U I R E D* ⚡\n\n" +
				"Champion, the following priority objective requires your expertise:\n\n" +
				"🎯 *" + r.Desc + "*\n\n" +
				"Excellence lies in execution. Bring your best to the field today and let's continue pioneering the standard in Smart Automation! 🚀\n\n" +
				"Regards,\n*" + os.Getenv("COMPANY_NAME") + " Team*"
			sendFAQAnswer(r.Phone, msg)
			db.MarkReminderSent(r.ID)
		}
	})
	if err != nil {
		log.Println("Error scheduling reminders checker:", err)
	}

	// ── Every minute: check for due campaigns and broadcast ──────────────────
	_, err = c.AddFunc("* * * * *", func() {
		campaigns, err := db.GetDueCampaigns()
		if err != nil {
			log.Println("[Scheduler] Error fetching due campaigns:", err)
			return
		}
		if len(campaigns) == 0 {
			return
		}

		phones, err := db.GetAllPhoneNumbers()
		if err != nil || len(phones) == 0 {
			log.Println("[Scheduler] No contacts to broadcast to")
			return
		}

		for _, camp := range campaigns {
			log.Printf("[Scheduler] Broadcasting campaign #%d (%s) to %d contacts", camp.ID, camp.Type, len(phones))

			switch strings.ToLower(camp.Type) {
			case "quiz":
				broadcastQuiz(camp, phones)
			case "poster":
				broadcastPoster(camp, phones)
			}

			db.MarkCampaignSent(camp.ID, len(phones))
		}
	})
	if err != nil {
		log.Println("Error scheduling campaign broadcaster:", err)
	}

	c.Start()
}

// Removed duplicate sendMorningCheckIn (now in internal.go)

func broadcastQuiz(camp db.Campaign, phones []string) {
	quizBody := fmt.Sprintf(
		"🌟 *%s Industrial Insight* 🌟\n\n"+
			"*WEEKLY KNOWLEDGE CHALLENGE*\n\n"+
			"❓ *QUESTION:*\n%s?\n\n"+
			"📍 *OPTIONS:*\n"+
			"📍 *A:* %s\n"+
			"📍 *B:* %s\n"+
			"📍 *C:* %s\n\n"+
			"👉 *Tap your answer below to participate!*",
		camp.Question, camp.OptionA, camp.OptionB, camp.OptionC,
	)

	buttons := []Button{
		{ID: "A", Title: "Option A"},
		{ID: "B", Title: "Option B"},
		{ID: "C", Title: "Option C"},
	}

	// Deactivate any previous quiz first
	db.DeactivateAllQuizzes()

	// Create quiz entry from campaign
	quizID, err := db.CreateQuizFromCampaign(camp)
	if err != nil {
		log.Printf("[Scheduler] Failed to create quiz entry: %v", err)
		return
	}
	log.Printf("[Scheduler] Activated quiz #%d", quizID)

	for _, phone := range phones {
		sendInteractiveButtons(phone, quizBody, buttons)

		// ENGAGEMENT TRIGGER: Wait exactly 2 minutes, then send the premium engagement message
		go func(p string) {
			time.Sleep(2 * time.Minute)
			log.Printf("[Scheduler] Sending 2-min engagement nudge to %s", p)
			sendEngagementNudge(p)
		}(phone)
	}
}

func broadcastPoster(camp db.Campaign, phones []string) {
	publicURL := os.Getenv("PUBLIC_URL")
	actualImageURL := camp.ImageURL

	// If the image is stored locally, we MUST use the public ngrok URL for Meta to reach it
	if strings.Contains(actualImageURL, "localhost") || strings.HasPrefix(actualImageURL, "/uploads") {
		filename := filepath.Base(actualImageURL)
		actualImageURL = fmt.Sprintf("%s/uploads/%s", publicURL, filename)
		log.Printf("[Scheduler] Local image detected. Rewriting to public: %s", actualImageURL)
	}

	premiumCaption := fmt.Sprintf(
		"📢 *%s Industrial Update* 📢\n"+
			"──────────────────⬡\n\n"+
			"%s\n\n"+
			"──────────────────⬡\n"+
			"🌐 *Visit us:* www.askworx.in\n"+
			"📧 *Support:* contact@askworx.in",
		camp.Caption,
	)

	buttons := []Button{
		{ID: "expert", Title: "Talk to Expert 📞"},
		{ID: "menu", Title: "Main Menu 🏠"},
	}

	for _, phone := range phones {
		sendImageWithButtons(phone, actualImageURL, premiumCaption, buttons)
	}
}
