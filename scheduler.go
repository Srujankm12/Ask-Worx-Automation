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

	// Morning Check-in for Internal Team (Nudge at 9 AM IST).
	// Only employees who messaged in the last 24 hours: the greeting is not a
	// template, so Meta rejects it for everyone else.
	_, err = c.AddFunc("0 9 * * *", func() {
		emps, err := db.GetEmployeesInServiceWindow()
		if err != nil {
			log.Println("[Scheduler] Error fetching employee greeting recipients:", err)
			return
		}
		log.Printf("[Scheduler] Sending employee greeting to %d employees in the 24h window", len(emps))
		for _, e := range emps {
			template := db.GetSetting("greeting_employee")
			if template == "" {
				template = "🌅 *Good Morning, {{name}}!* 🏆\n\nAnother day to pioneer industrial excellence. Don't forget to **Start Your Day** in the Internal Hub to log your focus objectives.\n\nLet's make an impact! 🚀"
			}
			msg := strings.ReplaceAll(template, "{{name}}", e.Name)
			sendEmployeeDashboard(e.Phone) // This will trigger the dashboard buttons
			sendTextMessage(e.Phone, msg)
		}
	})

	// Good Morning greeting for Users/Customers (Nudge at 8:30 AM IST).
	// Only contacts who messaged in the last 24 hours: the greeting is not a
	// template, so Meta rejects it for everyone else.
	_, err = c.AddFunc("30 8 * * *", func() {
		phones, err := db.GetPhonesInServiceWindow()
		if err != nil {
			log.Println("[Scheduler] Error fetching greeting recipients:", err)
			return
		}
		log.Printf("[Scheduler] Sending morning greeting to %d contacts in the 24h window", len(phones))
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
				{ID: "main_menu", Title: db.ButtonLabel("main_menu", "🏠 Main Menu")},
				{ID: "opt_out", Title: db.ButtonLabel("opt_out", "🛑 Stop Messages")},
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
			// {{task}} is specific to this template, so it is substituted here
			// rather than in renderTemplate, which only knows the three
			// placeholders every message shares.
			msg := strings.ReplaceAll(
				renderTemplate(db.SettingOr("emp_reminder",
					"*Reminder*\n\n{{task}}\n\n— {{company}}"),
					db.GetEmployeeName(r.Phone)),
				"{{task}}", r.Desc)
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

		// Only contacts inside the 24h window: campaigns are not templates, so
		// Meta rejects them for everyone else.
		phones, err := db.GetPhonesInServiceWindow()
		if err != nil {
			log.Println("[Scheduler] Error fetching campaign recipients:", err)
			return
		}

		for _, camp := range campaigns {
			// Claim it first. Sending takes longer than the one-minute tick,
			// so without this the next run picks the same row up and
			// broadcasts the whole campaign again.
			claimed, err := db.ClaimCampaign(camp.ID)
			if err != nil {
				log.Printf("[Scheduler] Could not claim campaign #%d, skipping: %v", camp.ID, err)
				continue
			}
			if !claimed {
				continue
			}

			log.Printf("[Scheduler] Broadcasting campaign #%d (%s) to %d contacts in the 24h window", camp.ID, camp.Type, len(phones))

			// With nobody in the window the campaign is still marked sent, to
			// 0. Leaving it due would fire it at whatever minute the next
			// person happened to message, to that one person.
			if len(phones) > 0 {
				broadcastPoster(camp, phones)
			}

			if err := db.MarkCampaignSent(camp.ID, len(phones)); err != nil {
				log.Printf("[Scheduler] Campaign #%d was sent but could not be marked sent: %v", camp.ID, err)
			}
		}
	})
	if err != nil {
		log.Println("Error scheduling campaign broadcaster:", err)
	}

	c.Start()
}

// Removed duplicate sendMorningCheckIn (now in internal.go)

func broadcastPoster(camp db.Campaign, phones []string) {
	publicURL := os.Getenv("PUBLIC_URL")
	actualImageURL := camp.ImageURL

	// If the image is stored locally, we MUST use the public ngrok URL for Meta to reach it
	if strings.Contains(actualImageURL, "localhost") || strings.HasPrefix(actualImageURL, "/uploads") {
		filename := filepath.Base(actualImageURL)
		actualImageURL = fmt.Sprintf("%s/uploads/%s", publicURL, filename)
		log.Printf("[Scheduler] Local image detected. Rewriting to public: %s", actualImageURL)
	}

	// The image carries the message; the caption is just what was typed in the
	// panel, followed by how to get in touch. No banner heading, no rule
	// lines, and no reply buttons — a poster is an announcement, not a menu.
	//
	// Posters created before the title/description split only have Caption, so
	// those still send their original text rather than a blank body.
	body := camp.Caption
	if camp.Title != "" || camp.Description != "" {
		body = fmt.Sprintf("*%s*\n\n%s", camp.Title, camp.Description)
	}

	// What closes the message. A broadcast that set its own links uses those;
	// otherwise it falls back to the website and email from settings, which
	// the panel can edit, so the default is not frozen in this file.
	closing := strings.TrimSpace(camp.Links)
	if closing == "" {
		var lines []string
		if site := strings.TrimSpace(db.SettingOr("poster_website", "www.askworx.in")); site != "" {
			lines = append(lines, "🌐 "+site)
		}
		if email := strings.TrimSpace(db.SettingOr("poster_email", "contact@askworx.in")); email != "" {
			lines = append(lines, "📧 "+email)
		}
		closing = strings.Join(lines, "\n")
	}

	caption := strings.TrimSpace(body)
	if closing != "" {
		caption += "\n\n" + closing
	}

	for _, phone := range phones {
		sendImage(phone, actualImageURL, caption)
	}
}
