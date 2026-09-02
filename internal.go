package main

import (
	"askworx-whatsapp-bot/db"
	"fmt"
	"os"
	"strings"
)

// Helpers to bridge sessions map in handler.go
func updateSession(phone, state string) {
	sessions[phone] = SessionState(state)
	s := getSession(phone)
	s["state"] = state
	saveSession(phone, s)
}

func clearSession(phone string) {
	delete(sessions, phone)
	delete(internalSessions, phone)
}

// Internal storage for complex employee states
var internalSessions = map[string]map[string]interface{}{}

func getSession(phone string) map[string]interface{} {
	if internalSessions[phone] == nil {
		internalSessions[phone] = make(map[string]interface{})
	}
	return internalSessions[phone]
}

func saveSession(phone string, s map[string]interface{}) {
	internalSessions[phone] = s
}

// Bridge to sendInteractiveButtons
func sendButtons(phone, text string, btns []map[string]string) {
	var buttons []Button
	for _, b := range btns {
		buttons = append(buttons, Button{ID: b["id"], Title: b["title"]})
	}
	sendInteractiveButtons(phone, text, buttons)
}

const (
	MODE_TEST = "TEST"
)

// Main dispatcher for Internal System
func tryInternalSystem(phone, input, state string, lat, lng float64) bool {
	// ── 1. ACCESS CONTROL ──────────────────────────────────────────────────
	allowed, _ := db.IsEmployee(phone)
	if !allowed {
		return false
	}

	// ── 2. MENU TRIGGER ────────────────────────────────────────────────────
	lower := strings.ToLower(input)
	if lower == "hi" || lower == "hey" || lower == "hello" || lower == "menu" || lower == "help" {
		sendEmployeeDashboard(phone)
		updateSession(phone, "internal_menu")
		return true
	}

	// ── 3. STATE HANDLING & DIRECT ACTIONS ─────────────────────────────────
	// Check for location data for attendance
	if input == "LOCATION_DATA" {
		if state == "expect_location_checkin" {
			db.MarkCheckIn(phone, lat, lng)
			sendTextMessage(phone, db.SettingOr("emp_checkin_done", "📍 *Location received.*\n\nYour attendance is recorded.\n\n*What are you working on today?*\nList your main tasks below."))
			updateSession(phone, "submit_workplan")
			return true
		}
		if state == "expect_location_checkout" {
			db.MarkCheckOut(phone, lat, lng)
			sendTextMessage(phone, db.SettingOr("emp_checkout_done", "📍 *Location received.*\n\nYour departure is recorded.\n\n*What did you complete today?*\nSend your end-of-day report below."))
			updateSession(phone, "submit_eod")
			return true
		}
	}

	// Check for direct button triggers first (high priority)
	if handleInternalMenu(phone, input) {
		return true
	}

	// Then handle complex flows based on state
	switch {
	case strings.HasPrefix(state, "leave_request"):
		return handleLeaveRequest(phone, input)
	case state == "submit_workplan":
		return handleWorkPlanSubmission(phone, input)
	case state == "submit_eod":
		return handleEODSubmission(phone, input)
	}

	return false
}

func sendEmployeeDashboard(phone string) {
	msg := renderTemplate(db.SettingOr("hub_welcome",
		"*{{company}} INTERNAL HUB*\n\nWelcome back, {{name}}.\n\nChoose an action below."),
		db.GetEmployeeName(phone))

	btnStart := db.GetSetting("btn_start_day")
	if btnStart == "" {
		btnStart = " 🏢 START DAY"
	}
	btnEnd := db.GetSetting("btn_end_day")
	if btnEnd == "" {
		btnEnd = "🏢 END DAY"
	}
	btnLeave := db.GetSetting("btn_apply_leave")
	if btnLeave == "" {
		btnLeave = "🏝️ APPLY LEAVE"
	}

	buttons := []map[string]string{
		{"id": "checkin", "title": btnStart},
		{"id": "checkout", "title": btnEnd},
		{"id": "leave_init", "title": btnLeave},
	}

	db.SaveMessageHistory(phone, msg, "outbound")
	sendButtons(phone, msg, buttons)
}

func handleInternalMenu(phone, input string) bool {
	// Handle both ID and formatted titles with emojis
	cleanInput := strings.ToLower(strings.TrimSpace(input))

	isCheckIn := cleanInput == "checkin" || strings.Contains(cleanInput, "start day")
	isCheckOut := cleanInput == "checkout" || strings.Contains(cleanInput, "end day")
	isLeave := cleanInput == "leave_init" || strings.Contains(cleanInput, "apply leave")

	switch {
	case isCheckIn:
		alreadyChecked, _ := db.HasCheckedInToday(phone)
		if alreadyChecked {
			sendTextMessage(phone, db.SettingOr("emp_checkin_already", "You have already checked in today."))
			return true
		}
		sendTextMessage(phone, db.SettingOr("emp_checkin_prompt", "*Share your location to check in.*\n\nUse the attachment button in WhatsApp and send your current location."))
		updateSession(phone, "expect_location_checkin")
		return true

	case isCheckOut:
		alreadyOut, _ := db.HasCheckedOutToday(phone)
		if alreadyOut {
			sendTextMessage(phone, db.SettingOr("emp_checkout_already", "You have already checked out today. Your report is filed."))
			return true
		}
		sendTextMessage(phone, db.SettingOr("emp_checkout_prompt", "*Share your location to check out.*\n\nUse the attachment button in WhatsApp and send your current location."))
		updateSession(phone, "expect_location_checkout")
		return true

	case isLeave:
		msg := db.SettingOr("emp_leave_type_prompt", "*Leave request*\n\nWhich type of leave do you need?")
		buttons := []map[string]string{
			{"id": "leave_casual", "title": db.ButtonLabel("leave_casual", "🛋️ CASUAL")},
			{"id": "leave_sick", "title": db.ButtonLabel("leave_sick", "🤒 SICK LEAVE")},
			{"id": "leave_emergency", "title": db.ButtonLabel("leave_emergency", "🚨 EMERGENCY")},
		}
		sendButtons(phone, msg, buttons)
		updateSession(phone, "leave_request_type")
		return true
	}
	return false
}

func handleLeaveRequest(phone, input string) bool {
	session := getSession(phone)
	state := session["state"].(string)

	switch state {
	case "leave_request_type":
		if strings.HasPrefix(input, "leave_") {
			session["leave_type"] = input
			session["state"] = "leave_request_date"
			saveSession(phone, session)
			sendTextMessage(phone, db.SettingOr("emp_leave_date_prompt", "*Which date is the leave for?*\nFor example: 20 Oct"))
			return true
		}
	case "leave_request_date":
		session["leave_date"] = input
		session["state"] = "leave_request_reason"
		saveSession(phone, session)
		sendTextMessage(phone, db.SettingOr("emp_leave_reason_prompt", "*What is the reason?*\nA short line is enough."))
		return true
	case "leave_request_reason":
		lType := fmt.Sprintf("%v", session["leave_type"])
		lDate := fmt.Sprintf("%v", session["leave_date"])

		db.SubmitLeave(phone, lType, lDate, input)

		msg := db.SettingOr("emp_leave_submitted", "*Leave request sent.*\n\nIt is with your manager now. You will get a message here once it is decided.")
		sendFAQAnswer(phone, msg)
		clearSession(phone)
		return true
	}

	return false
}

func handleWorkPlanSubmission(phone, input string) bool {
	db.UpdateWorkPlan(phone, input)
	msg := db.SettingOr("emp_workplan_saved", "*Day plan saved.* Have a good day.")
	sendFAQAnswer(phone, msg)
	clearSession(phone)
	return true
}

func handleEODSubmission(phone, input string) bool {
	db.UpdateEODReport(phone, input)
	msg := db.SettingOr("emp_eod_saved", "*End-of-day report filed.* Thank you — see you tomorrow.")
	sendFAQAnswer(phone, msg)
	clearSession(phone)
	return true
}

// Legacy flows removed.

// renderTemplate fills the placeholders a configurable message may contain.
//
// These are the only three, and they are documented in the admin panel beside
// every field that accepts them. An unknown placeholder is left as written
// rather than blanked, so a typo is visible in the sent message instead of
// silently deleting half a sentence.
//
// {{name}} falls back to a neutral form when the recipient is not a known
// employee, so a template never greets somebody as an empty string.
func renderTemplate(text, name string) string {
	if strings.TrimSpace(name) == "" {
		name = "there"
	}
	r := strings.NewReplacer(
		"{{company}}", os.Getenv("COMPANY_NAME"),
		"{{name}}", name,
		"{{phone}}", os.Getenv("ADMIN_PHONE"),
	)
	return r.Replace(text)
}
