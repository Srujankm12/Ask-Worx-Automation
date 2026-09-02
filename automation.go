package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"askworx-whatsapp-bot/db"
)

// ─── SESSION STATES FOR AUTOMATION ───────────────────────────────────────────

const (
	StateQueryCategory   SessionState = "query_category"
	StateQuizExplanation SessionState = "quiz_explanation"
)

// pendingMessages stores the user's original message while they select a category
var pendingMessages = map[string]string{}

// quizSessionStore stores which quiz the user is currently interacting with in the conversational flow
var quizSessionStore = map[string]*db.Quiz{}

// ─── PRIORITY DISPATCHER ──────────────────────────────────────────────────────
func tryAutomationModules(phone, rawInput string) bool {
	upper := strings.ToUpper(strings.TrimSpace(rawInput))

	// ── Priority 1: Quiz Explanation Flow (Ongoing conversational state) ─────
	if sessions[phone] == StateQuizExplanation {
		handleQuizExplanationReply(phone, upper)
		return true
	}

	// ── Priority 2: Active Quiz — ONLY handle if they type A, B, or C ────────
	quiz, err := db.GetActiveQuiz()
	if err != nil {
		log.Printf("[Quiz] DB error: %v", err)
	}
	if quiz != nil {
		answered, _ := db.HasUserResponded(quiz.ID, phone)
		if !answered {
			isA := (upper == "A" || strings.Contains(upper, "OPTION A") || upper == "1")
			isB := (upper == "B" || strings.Contains(upper, "OPTION B") || upper == "2")
			isC := (upper == "C" || strings.Contains(upper, "OPTION C") || upper == "3")

			if isA || isB || isC {
				ans := "A"
				if isB {
					ans = "B"
				}
				if isC {
					ans = "C"
				}
				handleQuizResponse(phone, ans, quiz)
				return true
			}
			// Important: If it's NOT A, B, or C, we just return false
			// so the main handler can show the Menu. No more "Selection Force".
		}
	}

	// ── Priority 3: FAQ Knowledge Base ───────────────────────────────────────
	if ans, confident := tryFAQMatch(rawInput); confident {
		sendFAQAnswer(phone, ans)
		return true
	}

	// ── Priority 4: Query category selection (ongoing Module 2 session) ──────
	if sessions[phone] == StateQueryCategory {
		handleQueryCategoryReply(phone, upper, rawInput)
		return true
	}

	return false
}

// ─── MODULE 1: QUIZ FLOW ──────────────────────────────────────────────────────

func handleQuizResponse(phone, answer string, quiz *db.Quiz) {
	isCorrect := answer == strings.ToUpper(quiz.CorrectAnswer)

	if err := db.SaveQuizResponse(quiz.ID, phone, answer, isCorrect); err != nil {
		log.Printf("[Quiz] Save error: %v", err)
	}

	var msg string
	if isCorrect {
		msg = fmt.Sprintf("🎉 *Thank you for participating in the %s Weekly Knowledge Challenge!*\n\n✅ *Awesome! That's the correct answer.*\n\nWould you like to see the detailed explanation? Tap below! 👇", os.Getenv("COMPANY_NAME"))
	} else {
		msg = fmt.Sprintf("🎉 *Thank you for participating in the %s Weekly Knowledge Challenge!*\n\n❌ *Oops! That was a tricky one, but it's not quite correct.*\n\nWould you like to see the right answer and explanation? Tap below! 👇", os.Getenv("COMPANY_NAME"))
	}

	// Store quiz and update state
	quizSessionStore[phone] = quiz
	sessions[phone] = StateQuizExplanation

	// Send as buttons for even easier interaction
	buttons := []Button{
		{ID: "YES", Title: "Yes, Explain"},
		{ID: "NO", Title: "No, Later"},
	}
	sendInteractiveButtons(phone, msg, buttons)
}

func handleQuizExplanationReply(phone, upper string) {
	quiz := quizSessionStore[phone]
	if quiz == nil {
		sessions[phone] = StateMain
		return
	}

	if upper == "YES" {
		msg := fmt.Sprintf(
			"🎯 *The Correct Answer is: %s*\n\n%s\n\n🎥 Watch video: %s",
			strings.ToUpper(quiz.CorrectAnswer),
			quiz.Explanation,
			quiz.YouTubeLink,
		)
		sendTextMessage(phone, msg)
	} else if upper == "NO" {
		sendTextMessage(phone, "👍 No problem! Stay tuned for next week's quiz.")
	} else {
		// Invalid response while in this state
		sendTextMessage(phone, "Please reply *YES* or *NO* to see the quiz explanation.")
		return
	}

	// Reset state
	sessions[phone] = StateMain
	delete(quizSessionStore, phone)
}

// StartQueryFlow initiates the support assistant flow — called when no quiz/FAQ matched.
func StartQueryFlow(phone, originalMessage string) {
	log.Printf("[Support] Starting assistant flow for %s", phone)

	// Step 2: Auto Response
	greeting := fmt.Sprintf("Hi! 👋 Thanks for reaching out to *%s*.\n\nWe've received your message and our team will assist you shortly.", os.Getenv("COMPANY_NAME"))
	sendTextMessage(phone, greeting)

	// Step 3: Send Button Menu
	body := "Please select your query type:"
	buttons := []Button{
		{ID: "service", Title: db.ButtonLabel("service", "🔧 Service Request")},
		{ID: "quotation", Title: db.ButtonLabel("quotation", "💰 Get a Quote")},
		{ID: "product", Title: db.ButtonLabel("product", "🛠️ Product Query")},
		{ID: "general", Title: db.ButtonLabel("general", "💬 General Inquiry")},
	}
	sendInteractiveButtons(phone, body, buttons)

	sessions[phone] = StateQueryCategory
	pendingMessages[phone] = originalMessage
}

// ─── MODULE 2: PREMIUM SUPPORT HANDLER ────────────────────────────────────────

var CategoryLabels = map[string]string{
	"service":   "Service Request",
	"quotation": "Request Quotation",
	"technical": "Technical Query",
	"general":   "General Inquiry",
}

func handleQueryCategoryReply(phone, upper, rawInput string) {
	// Match against the ID (lowercase rawInput from buttons)
	label, valid := CategoryLabels[strings.ToLower(rawInput)]
	if !valid {
		// Edge case: Remind user to use buttons
		sendTextMessage(phone, "Please select one of the options above 👆")
		return
	}

	originalMsg := pendingMessages[phone]

	// Step 5: Store Query
	db.SaveCustomerQuery(db.CustomerQuery{
		PhoneNumber:     phone,
		UserName:        "Customer",
		OriginalMessage: originalMsg,
	})

	// Get the ID of the query to update category
	q, _ := db.GetLatestPendingQuery(phone)
	if q.ID != 0 {
		db.UpdateQueryCategory(q.ID, label)
	}

	// Step 7: Notify Internal Team
	NotifyTeam(phone, label, originalMsg)

	// Step 6: Confirm to User
	sendTextMessage(phone, "✅ Got it! Our team will contact you shortly regarding your request.")

	// Step 8: Reset State
	sessions[phone] = StateMain
	delete(pendingMessages, phone)
}

func NotifyTeam(phone, category, message string) {
	teamNumber := os.Getenv("TEAM_WHATSAPP_NUMBER")
	if teamNumber == "" {
		log.Println("[Support] TEAM_WHATSAPP_NUMBER not set — skipping notification")
		return
	}

	notification := fmt.Sprintf(
		"📩 *New Customer Query*\n\n👤 *Number:* +%s\n📂 *Category:* %s\n💬 *Message:* %s",
		phone, category, message,
	)
	sendTextMessage(teamNumber, notification)
}

// tryFAQMatch returns (answer, isConfident)
func tryFAQMatch(input string) (string, bool) {
	lower := strings.ToLower(input)

	// Skip button IDs — they contain underscores and should not be matched by FAQ
	if strings.Contains(lower, "_") {
		return "", false
	}

	faqs, err := db.GetAllFAQs()
	if err != nil {
		log.Printf("[FAQ] DB error: %v", err)
		return "", false
	}

	for _, entry := range faqs {
		keywords := strings.Split(entry.Keywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
				return entry.Answer, true
			}
		}
	}
	return "", false
}

// sendEngagementNudge sends a professional business introduction to convert
// quiz interest into service inquiries.
func sendEngagementNudge(phone string) {
	greeting := fmt.Sprintf("👋 Welcome to %s.\n\n"+
		"We are a Ground-to-Cloud automation company helping businesses with industrial automation, digital transformation, and smart engineering solutions.\n\n"+
		"From PLC, SCADA, and IIoT systems to software development, CRM solutions, and digital marketing — we provide complete end-to-end solutions.\n\n"+
		"How can we assist you today?", os.Getenv("COMPANY_NAME"))

	buttons := []Button{
		{ID: "flow_service", Title: db.ButtonLabel("flow_service", "🔧 Service Request")},
		{ID: "flow_quotation", Title: db.ButtonLabel("flow_quotation", "💰 Get a Quote")},
		{ID: "flow_callback", Title: db.ButtonLabel("flow_callback", "📞 Book Callback")},
	}

	sendInteractiveButtons(phone, greeting, buttons)
}
