package main

import (
	"fmt"
	"os"
	"strings"

	"askworx-whatsapp-bot/db"
)

type SessionState string

const (
	StateMain             SessionState = "main"
	StateSolutions        SessionState = "solutions"
	StateIndustrial       SessionState = "industrial"
	StatePLC              SessionState = "plc"
	StateSCADA            SessionState = "scada"
	StateRobotics         SessionState = "robotics"
	StateControlPanel     SessionState = "control_panel"
	StateSoftware         SessionState = "software"
	StateSoftwareSub      SessionState = "software_sub"
	StateWhatsAppBot      SessionState = "whatsapp_bot"
	StateAppDev           SessionState = "app_dev"
	StateSEO              SessionState = "seo"
	StateIIoT             SessionState = "iiot"
	StateIIoTGateway      SessionState = "iiot_gateway"
	StateCloudAnalytics   SessionState = "cloud_analytics"
	StateQuote            SessionState = "quote"
	StateQuoteName        SessionState = "quote_name"
	StateQuoteCompany     SessionState = "quote_company"
	StateQuoteRequirement SessionState = "quote_requirement"
	StateQuotePhone       SessionState = "quote_phone"
	StateExpert           SessionState = "expert"
	StateCallback         SessionState = "callback"
	StateAbout            SessionState = "about"
	StateIndustries       SessionState = "industries"
	StateFAQ              SessionState = "faq"

	// Automation module states
	StateQueryCat SessionState = "query_category"

	// Lead Generation states
	StateLeadQuoteName     SessionState = "lead_quote_name"
	StateLeadQuoteCompany  SessionState = "lead_quote_company"
	StateLeadQuoteInterest SessionState = "lead_quote_interest"
	StateLeadQuoteDesc     SessionState = "lead_quote_desc"

	StateLeadCallName    SessionState = "lead_call_name"
	StateLeadCallCompany SessionState = "lead_call_company"
	StateLeadCallTime    SessionState = "lead_call_time"

	StateLeadServiceCategory SessionState = "lead_service_category"
	StateLeadServiceName     SessionState = "lead_service_name"
	StateLeadServiceCompany  SessionState = "lead_service_company"
	StateLeadServiceTime     SessionState = "lead_service_time"

	// Internal System states
	StateInternalWorkPlan    SessionState = "internal_work_plan"
	StateInternalEODReport   SessionState = "internal_eod_report"
	StateInternalLeaveDate   SessionState = "internal_leave_date"
	StateInternalLeaveReason SessionState = "internal_leave_reason"
)

var sessions = map[string]SessionState{}

type TempQuote struct {
	Name        string
	Company     string
	Requirement string
	Phone       string
}

type TempLead struct {
	Category    string
	Name        string
	Company     string
	Interest    string
	Description string
	Time        string
}

var tempQuotes = map[string]*TempQuote{}
var tempLeads = map[string]*TempLead{}
var tempCallbacks = map[string]string{}

func handleMessage(phone, input string, lat, lng float64) {
	text := strings.ToLower(strings.TrimSpace(input))
	db.SaveContact(phone, "")

	if text == "stop" || input == "opt_out" {
		db.UpdateContactOptOutByPhone(phone, true)
		sendTextMessage(phone, "✅ You have successfully opted out of our automated broadcasts and daily messages. If you ever need assistance, you can always type 'hi' or 'menu'.")
		return
	}

	if text == "subscribe" || input == "opt_in" {
		db.UpdateContactOptOutByPhone(phone, false)
		sendTextMessage(phone, "✅ You have been successfully re-subscribed to our automated updates.")
		return
	}

	// --- Module 4: Internal Team System (Priority 0) ---
	if tryInternalSystem(phone, input, string(sessions[phone]), lat, lng) {
		return
	}

	// ── ROLE ISOLATION ──────────────────────────────────────────────────────
	// If it's a registered employee, do NOT allow any customer flows below
	isEmp, _ := db.IsEmployee(phone)
	if isEmp {
		// Log internal message for dashboard history
		db.SaveMessageHistory(phone, text, "inbound")

		// Map total menu resets to the Internal Hub only
		if text == "hi" || text == "hello" || text == "hey" || text == "start" || text == "menu" || input == "main_menu" ||
			strings.Contains(text, "need assistance with your industrial platform") {
			sendEmployeeDashboard(phone)
		} else {
			// If not handled by tryInternalSystem and not a menu reset, show menu
			sendEmployeeDashboard(phone)
		}
		return
	}

	// ── Priority 1: Global Command Overrides ─────────────────────────────────
	if text == "hi" || text == "hello" || text == "hey" || text == "start" || text == "menu" || 
		strings.Contains(text, "main menu") || input == "main_menu" || 
		strings.Contains(text, "need assistance with your industrial platform") {
		db.UpdateContactOptOutByPhone(phone, false) // Auto-subscribe on interaction
		sendOpeningMessage(phone)
		return
	}

	// Gratitude / Thank You Handler
	if text == "thanks" || text == "thank you" || text == "thx" || text == "tq" || 
		text == "tha k you" || text == "thk you" || text == "thankyou" || text == "great thanks" {
		msg := fmt.Sprintf("🙏 You're most welcome! I'm glad I could assist you.\n\nI think you are satisfied with the information. If you need *urgent contact*, please reach out to our team directly at:\n\n📞 *+%s*\n\nHave a wonderful day! 😊", os.Getenv("ADMIN_PHONE"))
		buttons := []Button{
			{ID: "main_menu", Title: "🏠 Main Menu"},
			{ID: "talk_to_expert", Title: "📞 Talk to Expert"},
		}
		sendInteractiveButtons(phone, msg, buttons)
		return
	}
	if text == "help" {
		sendHelpMessage(phone)
		return
	}
	if text == "services" || text == "solutions" || strings.Contains(text, "our solutions") || input == "our_solutions" {
		sendSolutionsMenu(phone)
		return
	}
	if text == "contact" || strings.Contains(text, "talk to expert") || input == "talk_to_expert" {
		sendExpertContact(phone)
		return
	}
	if text == "about" || strings.Contains(text, "about "+strings.ToLower(os.Getenv("COMPANY_NAME"))) || input == "about_askworx" {
		sendAboutASKworX(phone)
		return
	}
	if text == "industries" || strings.Contains(text, "our industries") || input == "our_industries" {
		sendIndustriesPage(phone)
		return
	}
	if text == "faq" || strings.Contains(text, "support") || input == "support_faq" {
		sendFAQPrompt(phone)
		return
	}

	// ── Priority 2: Automation Modules (Quiz → FAQ → Query) ──────────────────
	if tryAutomationModules(phone, input) {
		return
	}

	// Engagement Nudge Overrides
	if text == "flow_service" {
		startLeadServiceFlow(phone)
		return
	}
	if text == "flow_quotation" {
		startLeadQuoteFlow(phone)
		return
	}
	if text == "flow_callback" {
		startLeadCallFlow(phone)
		return
	}

	state := sessions[phone]
	if state == "" {
		state = StateMain
	}

	switch state {
	case StateLeadQuoteName:
		handleLeadQuoteName(phone, input)
	case StateLeadQuoteCompany:
		handleLeadQuoteCompany(phone, input)
	case StateLeadQuoteInterest:
		handleLeadQuoteInterest(phone, input)
	case StateLeadQuoteDesc:
		handleLeadQuoteDesc(phone, input)
	case StateLeadCallName:
		handleLeadCallName(phone, input)
	case StateLeadCallCompany:
		handleLeadCallCompany(phone, input)
	case StateLeadCallTime:
		handleLeadCallTime(phone, input)
	case StateLeadServiceCategory:
		handleLeadServiceCategory(phone, input)
	case StateLeadServiceName:
		handleLeadServiceName(phone, input)
	case StateLeadServiceCompany:
		handleLeadServiceCompany(phone, input)
	case StateLeadServiceTime:
		handleLeadServiceTime(phone, input)
	case StateMain:
		handleMainFlow(phone, text)
	case StateSolutions:
		handleSolutionsFlow(phone, text)
	case StateIndustrial:
		handleIndustrialFlow(phone, text)
	case StatePLC, StateSCADA, StateRobotics, StateControlPanel:
		handleSubIndustrialFlow(phone, text)
	case StateSoftware:
		handleSoftwareFlow(phone, text)
	case StateSoftwareSub, StateWhatsAppBot, StateAppDev, StateSEO:
		handleSubSoftwareFlow(phone, text)
	case StateIIoT:
		handleIIoTFlow(phone, text)
	case StateIIoTGateway, StateCloudAnalytics:
		handleSubIIoTFlow(phone, text)
	case StateQuoteName:
		handleQuoteName(phone, input)
	case StateQuoteCompany:
		handleQuoteCompany(phone, input)
	case StateQuoteRequirement:
		handleQuoteRequirement(phone, input)
	case StateQuotePhone:
		handleQuotePhone(phone, input)
	case StateExpert:
		handleExpertFlow(phone, text)
	case StateCallback:
		handleCallbackRequest(phone, input)
	case StateAbout:
		handleAboutFlow(phone, text)
	case StateIndustries:
		handleIndustriesFlow(phone, text)
	case StateFAQ:
		handleFAQFlow(phone, input)
	case StateQueryCat:
		// handled by automation dispatcher (tryAutomationModules), shouldn't reach here
		sendOpeningMessage(phone)
	default:
		// Unknown input in main state → try Module 2 query flow
		StartQueryFlow(phone, input)
	}
}

// --- FLOW: OPENING ---
func sendOpeningMessage(phone string) {
	sessions[phone] = StateMain
	imageURL := db.GetSetting("content_welcome_image")
	if imageURL == "" {
		imageURL = "https://images.unsplash.com/photo-1581091226825-a6a2a5aee158?w=800"
	}
	body := db.GetSetting("content_welcome_body")
	if body == "" {
		body = fmt.Sprintf("🏭 Welcome to %s!\nGround to Cloud — Engineering Smart Automation\n\nWe help manufacturers move from shop-floor control to cloud-connected intelligence.\n\n🎯 Trusted by industries across India\n⚡ 24/7 Automation Support\n🌐 www.askworx.in\n\nHow can we assist you today?", os.Getenv("COMPANY_NAME"))
	}
	body = strings.ReplaceAll(body, "{{company}}", os.Getenv("COMPANY_NAME"))

	buttons := []Button{
		{ID: "our_solutions", Title: "🔧 Our Solutions"},
		{ID: "talk_to_expert", Title: "📞 Talk to Expert"},
		{ID: "about_askworx", Title: fmt.Sprintf("🏭 About %s", os.Getenv("COMPANY_NAME"))},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendHelpMessage(phone string) {
	msg := "🤖 *ASKworX Support Assistant*\n\nHow can I help you today? You can use the buttons below to navigate or type your query directly."
	buttons := []Button{
		{ID: "main_menu", Title: "🏠 Main Menu"},
		{ID: "talk_to_expert", Title: "📞 Talk to Expert"},
		{ID: "our_solutions", Title: "🔧 Our Solutions"},
	}
	sendInteractiveButtons(phone, msg, buttons)
}

func handleMainFlow(phone, text string) {
	switch text {
	case "our_solutions":
		sendSolutionsMenu(phone)
	case "talk_to_expert":
		sendExpertContact(phone)
	case "about_askworx":
		sendAboutASKworX(phone)
	case "quotation":
		startLeadQuoteFlow(phone)
	case "callback":
		startLeadCallFlow(phone)
	case "explore":
		sendExploreServices(phone)
	default:
		StartQueryFlow(phone, text)
	}
}

// --- FLOW: SOLUTIONS ---
func sendSolutionsMenu(phone string) {
	sessions[phone] = StateSolutions
	imageURL := "https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=800"
	body := "🔧 Our Solutions — Ground to Cloud Automation\nFrom sensor-level data to cloud intelligence, we engineer the future of manufacturing.\n\n✅ PLC & SCADA Systems\n✅ Industrial Networking & IIoT\n✅ Digital Transformation (Software/ERP)\n✅ ATEX Certified Industrial Products\n✅ AI-Powered Data Analytics\n\nWhat are you looking for?"
	buttons := []Button{
		{ID: "industrial_auto", Title: "⚙️ Industrial Auto"},
		{ID: "digital_software", Title: "💻 Digital & Software"},
		{ID: "iiot_analytics", Title: "📊 IIoT & Analytics"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleSolutionsFlow(phone, text string) {
	switch text {
	case "industrial_auto":
		sendIndustrialMenu(phone)
	case "digital_software":
		sendSoftwareMenu(phone)
	case "iiot_analytics":
		sendIIoTMenu(phone)
	default:
		sendSolutionsMenu(phone)
	}
}

// --- FLOW: INDUSTRIAL ---
func sendIndustrialMenu(phone string) {
	sessions[phone] = StateIndustrial
	imageURL := db.GetSetting("content_industrial_image")
	if imageURL == "" {
		imageURL = "https://images.unsplash.com/photo-1537462715879-360eeb61a0ad?w=800"
	}
	body := db.GetSetting("content_industrial_body")
	if body == "" {
		body = "⚙️ Industrial Automation\nThe Foundation: Total Control at Machine Level\n\nWe design, build, and commission high-reliability automation systems engineered for continuous 24/7 industrial operations. Our experts specialize in creating seamless machine-level interfaces that maximize uptime.\n\n🔹 End-to-End System Design\n🔹 Retrofitting & Upgrades\n🔹 Machine Monitoring\n🔹 High-Performance Algorithms\n🔹 Safety-First Engineering\n\nSelect a service to learn more:"
	}
	buttons := []Button{
		{ID: "plc_control", Title: "⚡ PLC & Control"},
		{ID: "scada_hmi", Title: "🖥️ SCADA & HMI"},
		{ID: "robotics", Title: "🤖 Robotics"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleIndustrialFlow(phone, text string) {
	switch text {
	case "plc_control":
		sendPLCDetails(phone)
	case "scada_hmi":
		sendSCADADetails(phone)
	case "robotics":
		sendRoboticsDetails(phone)
	default:
		sendIndustrialMenu(phone)
	}
}

func sendPLCDetails(phone string) {
	sessions[phone] = StatePLC
	imageURL := "https://images.unsplash.com/photo-1581094794329-c8112a89af12?w=800"
	body := "⚡ PLC & Control Systems\nComplete programmable control solutions for your plant floor.\n\n✅ Micro & Modular PLCs\n✅ Motion Controllers\n✅ Integrated PLC/HMI units\n✅ Variable Frequency Inverters (VFDs)\n✅ AC Servo Drive Systems\n✅ High-speed precision control\n✅ Safety PLC systems\n✅ Redundant control architecture\n\n🎯 Industries served:\nAutomotive | Pharma | Food & Beverage | Packaging | EV & Battery\n\n📞 Contact us for a free consultation and custom quote."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "next_to_scada", Title: "⏭️ Next Service"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendSCADADetails(phone string) {
	sessions[phone] = StateSCADA
	imageURL := "https://images.unsplash.com/photo-1551288049-bebda4e38f71?w=800"
	body := "🖥️ SCADA & HMI Development\nVisualize and control your entire operation from one screen.\n\n✅ Complete SCADA system development\n✅ MC Works64 & industry-standard platforms\n✅ HMI design for intuitive process control\n✅ Real-time monitoring & alarming\n✅ Historical data logging & trending\n✅ Multi-site remote monitoring\n✅ Custom reporting & dashboards\n✅ Mobile access to your plant data\n\n📊 Result: Complete plant visibility in real-time\n\n📞 Contact us for a free consultation and custom quote."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "next_to_robotics", Title: "⏭️ Next Service"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendRoboticsDetails(phone string) {
	sessions[phone] = StateRobotics
	imageURL := "https://images.unsplash.com/photo-1485827404703-89b55fcc595e?w=800"
	body := "🤖 Industrial Robotics & Motion\nThe Vanguard: Integrating Advanced Robotics\n\nTurnkey robot integration for modern manufacturing.\n\n✅ High-speed industrial robots\n✅ Collaborative Robots (Cobots)\n✅ Assembly & welding automation\n✅ Material handling systems\n✅ Precision multi-axis motion control\n✅ Vision-guided robotic systems\n✅ Robot programming & commissioning\n✅ After-sales support & training\n\n🎯 Applications:\nAssembly | Welding | Pick & Place | Inspection | Palletizing\n\n📞 Contact us for a free consultation and custom quote."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "next_to_panels", Title: "⏭️ Control Panels"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendPanelDetails(phone string) {
	sessions[phone] = StateControlPanel
	imageURL := "https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800"
	body := "🔌 Control Panel Design & Engineering\nThe Powerhouse: Reliable System Architecture\n\nIEC 61439 standard control panels built for reliability.\n\n✅ Complete panel architecture design\n✅ IEC 61439 international standard\n✅ Low-voltage circuit breakers & contactors\n✅ Motor starters & protection relays\n✅ Power Management Meters\n✅ Energy saving devices\n✅ Full documentation & testing\n✅ FAT & SAT support\n\n🏆 Built for 24/7 industrial operations\n\n📞 Contact us for a free consultation and custom quote."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "back_to_solutions", Title: "🔙 Back to Solutions"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleSubIndustrialFlow(phone, text string) {
	switch text {
	case "get_free_quote":
		startQuoteFlow(phone)
	case "next_to_scada":
		sendSCADADetails(phone)
	case "next_to_robotics":
		sendRoboticsDetails(phone)
	case "next_to_panels":
		sendPanelDetails(phone)
	case "back_to_solutions":
		sendSolutionsMenu(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendIndustrialMenu(phone)
	}
}

// --- FLOW: SOFTWARE ---
func sendSoftwareMenu(phone string) {
	sessions[phone] = StateSoftware
	imageURL := db.GetSetting("content_software_image")
	if imageURL == "" {
		imageURL = "https://images.unsplash.com/photo-1504868584819-f8e8b4b6d7e3?w=800"
	}
	body := db.GetSetting("content_software_body")
	if body == "" {
		body = "💻 Digital & Software Solutions\nBridging traditional automation with modern digital thinking to scale your business.\n\nFrom automated customer engagement to full-scale enterprise software, we engineer tools that drive growth."
	}
	buttons := []Button{
		{ID: "software_solutions", Title: "💻 Software Solutions"},
		{ID: "seo_marketing", Title: "📈 SEO & Marketing"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleSoftwareFlow(phone, text string) {
	switch text {
	case "software_solutions":
		sendSoftwareSolutionMenu(phone)
	case "seo_marketing":
		sendSEODetails(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendSoftwareMenu(phone)
	}
}

func sendSoftwareSolutionMenu(phone string) {
	sessions[phone] = StateSoftwareSub
	imageURL := "https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=800"
	body := "💻 Software Solutions\nCustom-engineered software systems to automate and scale your business operations.\n\nSelect a specialized solution:"
	buttons := []Button{
		{ID: "whatsapp_bot", Title: "📱 WhatsApp Bots"},
		{ID: "web_app_dev", Title: "🌐 Web & App Dev"},
		{ID: "industrial_sw", Title: "⚙️ Industrial Software"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendWhatsAppBotDetails(phone string) {
	sessions[phone] = StateWhatsAppBot
	imageURL := "https://images.unsplash.com/photo-1611746872915-64382b5c76da?w=800"
	body := "📱 WhatsApp Business Automation\nTransform your Customer Experience with 24/7 Intelligent Automation.\n\n✅ AI-Powered Conversation Flows\n✅ Full CRM & Database Integration\n✅ Automated Lead Qualification\n✅ Order Tracking & Payments\n✅ Multi-agent Admin Dashboard\n✅ Direct Broadcast Management\n\nScale your sales and support without adding headcount."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "web_app_dev", Title: "🌐 Web & App Dev"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendWebAppDetails(phone string) {
	sessions[phone] = StateAppDev
	imageURL := "https://images.unsplash.com/photo-1555066931-4365d14bab8c?w=800"
	body := "🌐 Web & Mobile Portfolio\nEnterprise-grade digital products designed for high performance.\n\n✅ Progressive Web Apps (PWA)\n✅ High-Speed Corporate Websites\n✅ Mobile Apps (Flutter, React Native)\n✅ Headless CMS Solutions\n✅ Serverless API Architecture\n✅ AWS/GCP Cloud Deployment\n\nBuilt for speed, security, and extreme scalability. 🚀"
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "industrial_sw", Title: "⚙️ Industrial Software"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendIndustrialSoftwareDetails(phone string) {
	sessions[phone] = StateSoftware
	imageURL := "https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=800"
	body := "⚙️ Industrial & ERP Software\nRobust backend systems to manage your shop floor and business.\n\n✅ Custom ERP & MES Systems\n✅ Real-time Inventory Tracking\n✅ Shop-floor Data Logging\n✅ Predictive Analytics Engines\n✅ Secure Cloud Dashboards\n✅ Desktop Process Monitors\n\nBridging the gap between machinery and business intelligence."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "seo_marketing", Title: "📈 Digital Marketing"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendSEODetails(phone string) {
	sessions[phone] = StateSEO
	imageURL := "https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=800"
	body := "📈 Digital Marketing & SEO\nDominating search results and driving high-intent traffic.\n\n✅ Data-Driven SEO Strategies\n✅ ROI-Focused Google Ads (PPC)\n✅ LinkedIn B2B Lead Generation\n✅ Social Media Brand Positioning\n✅ Content Authority Building\n✅ Advanced Analytics & Tracking\n\nWe don't just get traffic; we get paying customers."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "back_to_software", Title: "🔙 Software Menu"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleSubSoftwareFlow(phone, text string) {
	switch text {
	case "get_free_quote":
		startQuoteFlow(phone)
	case "whatsapp_bot":
		sendWhatsAppBotDetails(phone)
	case "web_app_dev":
		sendWebAppDetails(phone)
	case "industrial_sw":
		sendIndustrialSoftwareDetails(phone)
	case "seo_marketing":
		sendSEODetails(phone)
	case "back_to_software":
		sendSoftwareSolutionMenu(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendSoftwareSolutionMenu(phone)
	}
}

// --- FLOW: IIoT ---
func sendIIoTMenu(phone string) {
	sessions[phone] = StateIIoT
	imageURL := db.GetSetting("content_iiot_image")
	if imageURL == "" {
		imageURL = "https://images.unsplash.com/photo-1518770660439-4636190af475?w=800"
	}
	body := db.GetSetting("content_iiot_body")
	if body == "" {
		body = "📊 IIoT & Analytics Solutions\nConnecting your plant to the cloud.\n\nSelect a service:"
	}
	buttons := []Button{
		{ID: "iiot_gateway", Title: "🌐 IIoT Gateway"},
		{ID: "cloud_analytics", Title: "☁️ Cloud Analytics"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleIIoTFlow(phone, text string) {
	switch text {
	case "iiot_gateway":
		sendIIoTGatewayDetails(phone)
	case "cloud_analytics":
		sendCloudAnalyticsDetails(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendIIoTMenu(phone)
	}
}

func sendIIoTGatewayDetails(phone string) {
	sessions[phone] = StateIIoTGateway
	imageURL := "https://images.unsplash.com/photo-1518770660439-4636190af475?w=800"
	body := "🌐 IIoT Gateway Solutions\nThe Bridge: Connecting Your Plant to the Cloud\n\n✅ Industrial IoT gateway deployment\n✅ Machine-to-cloud connectivity\n✅ OPC-UA, Modbus, MQTT protocols\n✅ Secure encrypted data transfer\n✅ Edge computing solutions\n\n📞 Contact us for a free consultation."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "next_to_analytics", Title: "⏭️ Cloud Analytics"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func sendCloudAnalyticsDetails(phone string) {
	sessions[phone] = StateCloudAnalytics
	imageURL := "https://images.unsplash.com/photo-1551288049-bebda4e38f71?w=800"
	body := "☁️ Cloud Insights & Analytics\nTransforming Data into Operational Intelligence\n\n✅ Real-time production dashboards\n✅ OEE tracking & reporting\n✅ Predictive maintenance alerts\n✅ Energy consumption monitoring\n\n📊 Our customers achieve 99.9% visibility."
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "back_to_solutions", Title: "🔙 Back to Solutions"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleSubIIoTFlow(phone, text string) {
	switch text {
	case "get_free_quote":
		startQuoteFlow(phone)
	case "next_to_analytics":
		sendCloudAnalyticsDetails(phone)
	case "back_to_solutions":
		sendSolutionsMenu(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendIIoTMenu(phone)
	}
}

// --- FLOW: QUOTE (MULTI-STEP) ---
func startQuoteFlow(phone string) {
	sessions[phone] = StateQuoteName
	tempQuotes[phone] = &TempQuote{}
	body := "💬 Free Consultation Request\nOur experts will analyze your requirements and provide a detailed proposal within 24 hours. 🎯\n\nPlease share your *full name*:"
	sendTextMessage(phone, body)
}

func handleQuoteName(phone, input string) {
	// If input looks like a button ID (contains underscore), ignore it for name
	if strings.Contains(input, "_") {
		return
	}
	tempQuotes[phone].Name = input
	sessions[phone] = StateQuoteCompany
	msg := fmt.Sprintf("👋 Hello %s!\n\n🏢 What is your *company name* and *industry*?\n\nExample: ABC Manufacturing, Automotive", input)
	sendTextMessage(phone, msg)
}

func handleQuoteCompany(phone, input string) {
	tempQuotes[phone].Company = input
	sessions[phone] = StateQuoteRequirement
	msg := "📋 Briefly describe your *requirement*:\n\nExample:\n\"Need PLC automation for 3 production lines\"\n\"Want WhatsApp bot for our hotel\""
	sendTextMessage(phone, msg)
}

func handleQuoteRequirement(phone, input string) {
	tempQuotes[phone].Requirement = input
	sessions[phone] = StateQuotePhone
	msg := "📞 Your *best contact number*?\n(Our expert will call within 24 hours)"
	sendTextMessage(phone, msg)
}

func handleQuotePhone(phone, input string) {
	t := tempQuotes[phone]
	t.Phone = input

	fmt.Printf("📝 Saving Lead for %s: %s\n", t.Name, t.Phone)
	err := db.CreateLead(phone, t.Name, t.Company, t.Requirement, t.Phone)
	if err != nil {
		fmt.Printf("❌ Database Error: %v\n", err)
	}
	db.UpdateContactName(phone, t.Name)
	db.UpdateContactCompany(phone, t.Company)

	sessions[phone] = StateMain
	body := fmt.Sprintf("✅ Request Received Successfully!\n🎉 Thank you %s!\n\n📋 Summary:\n👤 Name: %s\n🏢 Company: %s\n📝 Requirement: %s\n📞 Contact: %s\n\n⏰ Our expert will contact you within 24 hours with a detailed proposal.\n\nThank you for choosing %s! 🙏", t.Name, t.Name, t.Company, t.Requirement, t.Phone, os.Getenv("COMPANY_NAME"))
	sendTextMessage(phone, body)

	delete(tempQuotes, phone)
	sendOpeningMessage(phone)
}

// --- FLOW: EXPERT / CALLBACK ---
func sendExpertContact(phone string) {
	sessions[phone] = StateExpert
	imageURL := "https://images.unsplash.com/photo-1600880292203-757bb62b4baf?w=800"
	body := fmt.Sprintf("📞 %s Support Center\nHow can we help you today?\n\nSelect a category below to connect with the right expert.", os.Getenv("COMPANY_NAME"))
	buttons := []Button{
		{ID: "service", Title: "🔧 Service Request"},
		{ID: "quotation", Title: "📈 Request Quotation"},
		{ID: "technical", Title: "🛠️ Technical Query"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleExpertFlow(phone, text string) {
	switch text {
	case "service", "quotation", "technical", "general":
		// Handle these categories using the Support Assistant flow
		StartCategoryQueryFlow(phone, text)
	case "main_menu":
		sendOpeningMessage(phone)
	case "about_askworx":
		sendAboutASKworX(phone)
	default:
		// Check if it's one of the other global buttons often present
		if strings.Contains(text, "solutions") {
			sendSolutionsMenu(phone)
		} else {
			sendExpertContact(phone)
		}
	}
}

func StartCategoryQueryFlow(phone, categoryID string) {
	label, ok := CategoryLabels[categoryID]
	if !ok {
		label = "Inquiry"
	}
	tempLeads[phone] = &TempLead{Category: label}
	sessions[phone] = StateLeadServiceCategory

	body := fmt.Sprintf("✅ *Selected:* %s\n\nWhich service area can we help you with?", label)
	buttons := []Button{
		{ID: "cat_automation", Title: "⚙️ Industrial Auto"},
		{ID: "cat_app_dev", Title: "💻 Digital & Software"},
		{ID: "cat_marketing", Title: "📊 IIoT & Analytics"},
	}
	sendInteractiveButtons(phone, body, buttons)
}

func startCallbackFlow(phone string) {
	sessions[phone] = StateCallback
	body := "📞 Schedule a Callback\nPlease share your *name* and *preferred time*:\n\nFormat: Name, Time\nExample: Rahul Sharma, Tomorrow 3PM"
	sendTextMessage(phone, body)
}

func handleCallbackRequest(phone, input string) {
	parts := strings.Split(input, ",")
	name := input
	timeStr := "As soon as possible"
	if len(parts) >= 2 {
		name = strings.TrimSpace(parts[0])
		timeStr = strings.TrimSpace(parts[1])
	}

	db.CreateCallback(phone, name, timeStr)
	sessions[phone] = StateMain
	body := fmt.Sprintf("✅ Callback Scheduled!\nOur expert will call you at the requested time.\n\n📞 We will call from: +%s\n\nThank you for choosing %s! 🙏", os.Getenv("ADMIN_PHONE"), os.Getenv("COMPANY_NAME"))
	buttons := []Button{
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendInteractiveButtons(phone, body, buttons)
	sendOpeningMessage(phone)
}

// --- FLOW: ABOUT / INDUSTRIES ---
func sendAboutASKworX(phone string) {
	sessions[phone] = StateAbout
	imageURL := db.GetSetting("content_welcome_image") // Reuse the welcome image or add a new one
	if imageURL == "" {
		imageURL = "https://images.unsplash.com/photo-1581091226825-a6a2a5aee158?w=800"
	}
	body := db.GetSetting("about_company")
	if body == "" {
		body = fmt.Sprintf("🏭 About %s\nGround to Cloud — Engineering Smart Automation\n\nAt %s, we bridge the gap between shop-floor machinery and cloud-connected intelligence. We provide high-reliability solutions for modern manufacturing.\n\n🛠 Our Solutions:\n✅ PLC & SCADA Engineering\n✅ Control Panel Design\n✅ Industrial Networking\n✅ Robotics & Motion Control\n✅ IIoT & Cloud Integration\n✅ Data Visualization\n\n⚲ Office Address:\n1381, 6th Main Road, 1st Phase, BEML Layout, 5th Stage, RR Nagar, Bangalore, Karnataka - 560098.\n\n📞 Contact Detail:\n☎︎ +%s\n📧 contact@askworx.in\n🌐 www.askworx.in", os.Getenv("COMPANY_NAME"), os.Getenv("COMPANY_NAME"), os.Getenv("ADMIN_PHONE"))
	}

	buttons := []Button{
		{ID: "our_industries", Title: "🏭 Our Industries"},
		{ID: "our_solutions", Title: "🔧 Our Solutions"},
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
	}
	sendImageWithButtons(phone, imageURL, body, buttons)
}

func handleAboutFlow(phone, text string) {
	switch text {
	case "our_industries":
		sendIndustriesPage(phone)
	case "our_solutions":
		sendSolutionsMenu(phone)
	case "get_free_quote":
		startQuoteFlow(phone)
	default:
		sendAboutASKworX(phone)
	}
}

func sendIndustriesPage(phone string) {
	sessions[phone] = StateIndustries
	body := "🏭 Industries We Serve\n🚗 Automotive | 🔋 EV | 💊 Pharma | 🍔 Food & Bev | 📦 Material Handling | 👕 Textiles | EMS | Oil & Gas\n\nWe work with ALL manufacturing sectors!"
	buttons := []Button{
		{ID: "get_free_quote", Title: "💬 Get Free Quote"},
		{ID: "our_solutions", Title: "🔧 Our Solutions"},
		{ID: "main_menu", Title: "🏠 Main Menu"},
	}
	sendInteractiveButtons(phone, body, buttons) // No image here, keeping it as interactive only
}

func handleIndustriesFlow(phone, text string) {
	switch text {
	case "get_free_quote":
		startQuoteFlow(phone)
	case "our_solutions":
		sendSolutionsMenu(phone)
	case "main_menu":
		sendOpeningMessage(phone)
	default:
		sendIndustriesPage(phone)
	}
}

// --- FLOW: LEAD SERVICE REQUEST ---

func startLeadServiceFlow(phone string) {
	sessions[phone] = StateLeadServiceCategory
	tempLeads[phone] = &TempLead{}
	body := "Here are our key services. What kind of service are you looking for?"

	buttons := []Button{
		{ID: "cat_automation", Title: "⚙️ Industrial/PLC"},
		{ID: "cat_app_dev", Title: "💻 Application Dev"},
		{ID: "cat_marketing", Title: "📈 Digital Marketing"},
	}
	sendInteractiveButtons(phone, body, buttons)
}

func handleLeadServiceCategory(phone, input string) {
	// Map button IDs to human readable strings
	interest := input
	if input == "cat_automation" {
		interest = "Industrial Automation / PLC / ATEX"
	}
	if input == "cat_app_dev" {
		interest = "Application & Software Development (CRM)"
	}
	if input == "cat_marketing" {
		interest = "Digital Marketing & SEO"
	}

	tempLeads[phone].Interest = interest
	sessions[phone] = StateLeadServiceName

	body := "Great choice! To connect you with the right expert, may I have your name?"
	sendTextMessage(phone, body)
}

func handleLeadServiceName(phone, input string) {
	if strings.Contains(input, "_") {
		return
	}
	tempLeads[phone].Name = input
	sessions[phone] = StateLeadServiceCompany
	body := "Company name?"
	sendTextMessage(phone, body)
}

func handleLeadServiceCompany(phone, input string) {
	tempLeads[phone].Company = input
	sessions[phone] = StateLeadServiceTime
	body := "Preferred time to connect?"
	sendTextMessage(phone, body)
}

func handleLeadServiceTime(phone, input string) {
	t, ok := tempLeads[phone]
	if !ok {
		sendOpeningMessage(phone)
		return
	}
	t.Time = input

	// Save to DB as Lead
	db.CreateLead(phone, t.Name, t.Company, t.Category+": "+t.Interest, t.Time)
	db.UpdateContactName(phone, t.Name)
	db.UpdateContactCompany(phone, t.Company)

	// Confirmation
	sendTextMessage(phone, "✅ Thank you! Our team will connect with you shortly.")

	// Notify Team
	NotifyTeam(phone, t.Category, fmt.Sprintf("Interested In: %s\nName: %s\nCompany: %s\nPreferred Time: %s\n\nLead generated from Expert Menu.", t.Interest, t.Name, t.Company, t.Time))

	sessions[phone] = StateMain
	delete(tempLeads, phone)
}

// --- FLOW: LEAD QUOTATION ---

func startLeadQuoteFlow(phone string) {
	sessions[phone] = StateLeadQuoteName
	tempLeads[phone] = &TempLead{}
	body := "📝 Sure! Please share a few details:\n\n1. Your Name"
	sendTextMessage(phone, body)
}

func handleLeadQuoteName(phone, input string) {
	if strings.Contains(input, "_") {
		return
	} // ignore button clicks if any
	tempLeads[phone].Name = input
	sessions[phone] = StateLeadQuoteCompany
	body := "2. Company Name"
	sendTextMessage(phone, body)
}

func handleLeadQuoteCompany(phone, input string) {
	tempLeads[phone].Company = input
	sessions[phone] = StateLeadQuoteInterest
	body := "3. Area of Interest (e.g., Automation, Software, Marketing)"
	sendTextMessage(phone, body)
}

func handleLeadQuoteInterest(phone, input string) {
	tempLeads[phone].Interest = input
	sessions[phone] = StateLeadQuoteDesc
	body := "4. Brief Project Description (2 lines)"
	sendTextMessage(phone, body)
}

func handleLeadQuoteDesc(phone, input string) {
	t := tempLeads[phone]
	t.Description = input

	// Confirmation
	sendTextMessage(phone, "✅ Thank you! Our team will review your requirements and share a quotation shortly.")

	// Notify Team
	NotifyTeam(phone, "Quotation", fmt.Sprintf("Interested in: %s\nProject: %s\nCompany: %s\nUser Name: %s", t.Interest, t.Description, t.Company, t.Name))

	sessions[phone] = StateMain
	delete(tempLeads, phone)
}

func startLeadCallFlow(phone string) {
	sessions[phone] = StateLeadCallName
	tempLeads[phone] = &TempLead{}
	body := "📞 Great! Please share:\n\n1. Your Name"
	sendTextMessage(phone, body)
}

func handleLeadCallName(phone, input string) {
	if strings.Contains(input, "_") {
		return
	} // ignore button clicks if any
	tempLeads[phone].Name = input
	sessions[phone] = StateLeadCallCompany
	body := "2. Company Name"
	sendTextMessage(phone, body)
}

func handleLeadCallCompany(phone, input string) {
	tempLeads[phone].Company = input
	sessions[phone] = StateLeadCallTime
	body := "3. Preferred Time for Call"
	sendTextMessage(phone, body)
}

func handleLeadCallTime(phone, input string) {
	t := tempLeads[phone]
	t.Time = input

	// Confirmation
	sendTextMessage(phone, "✅ Your callback request has been scheduled. Our team will contact you at your preferred time.")

	// Notify Team
	NotifyTeam(phone, "Callback", fmt.Sprintf("Time: %s\nCompany: %s\nUser Name: %s", t.Time, t.Company, t.Name))

	sessions[phone] = StateMain
	delete(tempLeads, phone)
}

func sendExploreServices(phone string) {
	body := "Here’s what we offer:\n\n" +
		"🔧 Industrial Automation\n" +
		"⚙️ PLC / SCADA / IIoT\n" +
		"🛠️ ATEX Products\n" +
		"💻 Software Development (CRM, ERP, Apps)\n" +
		"📈 Digital Marketing Solutions\n\n" +
		"Would you like to:"

	buttons := []Button{
		{ID: "quotation", Title: "1️⃣ Request Quotation"},
		{ID: "callback", Title: "2️⃣ Book a Callback"},
	}

	sendInteractiveButtons(phone, body, buttons)
}

func sendFAQPrompt(phone string) {
	sessions[phone] = StateFAQ
	msg := fmt.Sprintf("🤖 *%s Support Assistant*\n\nI can answer questions about our services, location, and technical capabilities.\n\n*Go ahead, ask me anything!* (e.g., 'What is SCADA?' or 'Where is your office?')", os.Getenv("COMPANY_NAME"))
	sendTextMessage(phone, msg)
}

func handleFAQFlow(phone, input string) {
	// Let FAQ match try again
	ans, confident := tryFAQMatch(input)
	if confident {
		sendFAQAnswer(phone, ans)
		sessions[phone] = StateMain // Reset to main after a successful FAQ match
		return
	}

	// Only trigger support flow for messages longer than 3 chars (prevents "okay", "what" spam)
	if len(strings.TrimSpace(input)) > 3 {
		sendTextMessage(phone, "Let me connect you with our team for this.")
		StartQueryFlow(phone, input)
	} else {
		// Just guide them back
		sendTextMessage(phone, "I'm sorry, I didn't quite catch that. Could you please provide a few more details so I can help you better?")
		sessions[phone] = StateMain // Reset to main if too short to be a valid query
	}
}

func sendFAQAnswer(phone, answer string) {
	buttons := []Button{
		{ID: "main_menu", Title: "Main Menu 🏠"},
	}
	sendInteractiveButtons(phone, answer, buttons)
}
