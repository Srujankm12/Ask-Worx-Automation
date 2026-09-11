package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Internal System Tables
func CreateInternalTables() {
	// Attendance table
	_, err := Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS attendance (
			id SERIAL PRIMARY KEY,
			phone TEXT NOT NULL,
			check_in TIMESTAMP,
			check_out TIMESTAMP,
			check_in_lat DOUBLE PRECISION,
			check_in_lng DOUBLE PRECISION,
			check_out_lat DOUBLE PRECISION,
			check_out_lng DOUBLE PRECISION,
			work_plan TEXT,
			eod_report TEXT,
			date DATE DEFAULT CURRENT_DATE,
			UNIQUE(phone, date)
		)
	`)
	if err != nil {
		log.Fatal("Error creating attendance table:", err)
	}

	// Dynamic column migration for existing tables
	_, _ = Pool.Exec(context.Background(), `
		ALTER TABLE attendance ADD COLUMN IF NOT EXISTS check_in_lat DOUBLE PRECISION;
		ALTER TABLE attendance ADD COLUMN IF NOT EXISTS check_in_lng DOUBLE PRECISION;
		ALTER TABLE attendance ADD COLUMN IF NOT EXISTS check_out_lat DOUBLE PRECISION;
		ALTER TABLE attendance ADD COLUMN IF NOT EXISTS check_out_lng DOUBLE PRECISION;
	`)

	// Leaves table
	_, err = Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS leave_requests (
			id SERIAL PRIMARY KEY,
			phone TEXT NOT NULL,
			leave_type TEXT,
			leave_date TEXT,
			reason TEXT,
			status TEXT DEFAULT 'Pending'
		)
	`)
	if err != nil {
		log.Fatal("Error creating leave_requests table:", err)
	}

	// Employees table
	_, err = Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS employees (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT NOT NULL UNIQUE,
			role TEXT DEFAULT 'employee'
		)
	`)
	if err != nil {
		log.Fatal("Error creating employees table:", err)
	}

	// Reminders table
	_, err = Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS reminders (
			id SERIAL PRIMARY KEY,
			employee_phone TEXT NOT NULL,
			description TEXT NOT NULL,
			due_at TIMESTAMP NOT NULL,
			status TEXT DEFAULT 'Pending'
		)
	`)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	// Settings table
	_, err = Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("Error creating settings table:", err)
	}

	// Seed default settings
	seedSettings()
}

func seedSettings() {
	defaults := map[string]string{
		"greeting_employee": fmt.Sprintf("🌅 *Good Morning, {{name}}!* 🏆\n\nAnother day to pioneer industrial excellence. Don't forget to **Start Your Day** in the Internal Hub to log your focus objectives.\n\nLet's make an impact! 🚀"),
		"greeting_customer": fmt.Sprintf("🌅 *Good Morning from %s!* 🏭\n\nWe hope you have a productive day ahead. If you need any assistance with Industrial Automation, IIoT, or Software solutions, we are just a message away. 🚀", os.Getenv("COMPANY_NAME")),
		"about_company":     fmt.Sprintf("🏭 About %s\nGround to Cloud — Engineering Smart Automation\n\nAt %s, we bridge the gap between shop-floor machinery and cloud-connected intelligence. We provide high-reliability solutions for modern manufacturing.\n\n🛠 Our Solutions:\n✅ PLC & SCADA Engineering\n✅ Control Panel Design\n✅ Industrial Networking\n✅ Robotics & Motion Control\n✅ IIoT & Cloud Integration\n✅ Data Visualization\n\n⚲ Office Address:\n1381, 6th Main Road, 1st Phase, BEML Layout, 5th Stage, RR Nagar, Bangalore, Karnataka - 560098.\n\n📞 Contact Detail:\n☎︎ +%s\n📧 contact@askworx.in\n🌐 www.askworx.in", os.Getenv("COMPANY_NAME"), os.Getenv("COMPANY_NAME"), os.Getenv("ADMIN_PHONE")),
		"hub_welcome":       fmt.Sprintf("*%s INTERNAL HUB*\n\nWelcome back, *Champion*! 🏆\n\nAt %s, we aren't just building automation; we are *pioneering the future* of industrial intelligence. 🏭✨\n\nYour expertise today moves the needle for industries worldwide. From Ground to Cloud, let's deliver excellence and show why %s is the leader in Smart Automation. 🚀\n\nReady to make an impact? Select an action below: 👇", os.Getenv("COMPANY_NAME"), os.Getenv("COMPANY_NAME"), os.Getenv("COMPANY_NAME")),
		"support_center":    fmt.Sprintf("📞 %s Support Center\nHow can we help you today?\n\nSelect a category below to connect with the right expert.", os.Getenv("COMPANY_NAME")),
		// ── The employee journey, in the order an employee meets it ──────
		// Every one of these was hardcoded in internal.go, scheduler.go or
		// admin.go, so the admin panel could not reach a single message a
		// colleague receives. They are plain by default: this is a tool
		// somebody uses at 8am on a site, not a motivational channel.
		"emp_checkin_prompt":      "*Share your location to check in.*\n\nUse the attachment button in WhatsApp and send your current location.",
		"emp_checkin_done":        "📍 *Location received.*\n\nYour attendance is recorded.\n\n*What are you working on today?*\nList your main tasks below.",
		"emp_checkin_already":     "You have already checked in today.",
		"emp_workplan_saved":      "*Day plan saved.* Have a good day.",
		"emp_checkout_prompt":     "*Share your location to check out.*\n\nUse the attachment button in WhatsApp and send your current location.",
		"emp_checkout_done":       "📍 *Location received.*\n\nYour departure is recorded.\n\n*What did you complete today?*\nSend your end-of-day report below.",
		"emp_checkout_already":    "You have already checked out today. Your report is filed.",
		"emp_eod_saved":           "*End-of-day report filed.* Thank you — see you tomorrow.",
		"emp_leave_type_prompt":   "*Leave request*\n\nWhich type of leave do you need?",
		"emp_leave_date_prompt":   "*Which date is the leave for?*\nFor example: 20 Oct",
		"emp_leave_reason_prompt": "*What is the reason?*\nA short line is enough.",
		"emp_leave_submitted":     "*Leave request sent.*\n\nIt is with your manager now. You will get a message here once it is decided.",
		"emp_leave_decision":      "*Leave request {{status}}*\n\nYour leave request has been {{status}}.\n\n— {{company}}",
		"emp_reminder":            "*Reminder*\n\n{{task}}\n\n— {{company}}",
		"emp_announcement":        "*Announcement*\n\n{{message}}\n\n— {{company}}",

		// ── Customer-facing pages ─────────────────────────────────────────
		// Every menu and service page a customer can reach. These were fixed
		// string literals in handler.go, which meant the longest and most
		// commercially important messages the bot sends were the only ones
		// nobody could edit without a deploy.
		"content_analytics_body":     "☁️ Cloud Insights & Analytics\nTransforming Data into Operational Intelligence\n\n✅ Real-time production dashboards\n✅ OEE tracking & reporting\n✅ Predictive maintenance alerts\n✅ Energy consumption monitoring\n\n📊 Our customers achieve 99.9% visibility.",
		"content_analytics_image":    "https://images.unsplash.com/photo-1551288049-bebda4e38f71?w=800",
		"content_expert_image":       "https://images.unsplash.com/photo-1600880292203-757bb62b4baf?w=800",
		"content_explore_body":       "Here’s what we offer:\n\n🔧 Industrial Automation\n⚙️ PLC / SCADA / IIoT\n🛠️ ATEX Products\n💻 Software Development (CRM, ERP, Apps)\n📈 Digital Marketing Solutions\n\nWould you like to:",
		"content_faqprompt_body":     "🤖 *{{company}} Support Assistant*\n\nI can answer questions about our services, location, and technical capabilities.\n\n*Go ahead, ask me anything!* (e.g., 'What is SCADA?' or 'Where is your office?')",
		"content_gateway_body":       "🌐 IIoT Gateway Solutions\nThe Bridge: Connecting Your Plant to the Cloud\n\n✅ Industrial IoT gateway deployment\n✅ Machine-to-cloud connectivity\n✅ OPC-UA, Modbus, MQTT protocols\n✅ Secure encrypted data transfer\n✅ Edge computing solutions\n\n📞 Contact us for a free consultation.",
		"content_gateway_image":      "https://images.unsplash.com/photo-1518770660439-4636190af475?w=800",
		"content_help_body":          "🤖 *ASKworX Support Assistant*\n\nHow can I help you today? You can use the buttons below to navigate or type your query directly.",
		"content_indsoftware_body":   "⚙️ Industrial & ERP Software\nRobust backend systems to manage your shop floor and business.\n\n✅ Custom ERP & MES Systems\n✅ Real-time Inventory Tracking\n✅ Shop-floor Data Logging\n✅ Predictive Analytics Engines\n✅ Secure Cloud Dashboards\n✅ Desktop Process Monitors\n\nBridging the gap between machinery and business intelligence.",
		"content_indsoftware_image":  "https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=800",
		"content_industries_body":    "🏭 Industries We Serve\n🚗 Automotive | 🔋 EV | 💊 Pharma | 🍔 Food & Bev | 📦 Material Handling | 👕 Textiles | EMS | Oil & Gas\n\nWe work with ALL manufacturing sectors!",
		"content_panels_body":        "🔌 Control Panel Design & Engineering\nThe Powerhouse: Reliable System Architecture\n\nIEC 61439 standard control panels built for reliability.\n\n✅ Complete panel architecture design\n✅ IEC 61439 international standard\n✅ Low-voltage circuit breakers & contactors\n✅ Motor starters & protection relays\n✅ Power Management Meters\n✅ Energy saving devices\n✅ Full documentation & testing\n✅ FAT & SAT support\n\n🏆 Built for 24/7 industrial operations\n\n📞 Contact us for a free consultation and custom quote.",
		"content_panels_image":       "https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800",
		"content_plc_body":           "⚡ PLC & Control Systems\nComplete programmable control solutions for your plant floor.\n\n✅ Micro & Modular PLCs\n✅ Motion Controllers\n✅ Integrated PLC/HMI units\n✅ Variable Frequency Inverters (VFDs)\n✅ AC Servo Drive Systems\n✅ High-speed precision control\n✅ Safety PLC systems\n✅ Redundant control architecture\n\n🎯 Industries served:\nAutomotive | Pharma | Food & Beverage | Packaging | EV & Battery\n\n📞 Contact us for a free consultation and custom quote.",
		"content_plc_image":          "https://images.unsplash.com/photo-1581094794329-c8112a89af12?w=800",
		"content_robotics_body":      "🤖 Industrial Robotics & Motion\nThe Vanguard: Integrating Advanced Robotics\n\nTurnkey robot integration for modern manufacturing.\n\n✅ High-speed industrial robots\n✅ Collaborative Robots (Cobots)\n✅ Assembly & welding automation\n✅ Material handling systems\n✅ Precision multi-axis motion control\n✅ Vision-guided robotic systems\n✅ Robot programming & commissioning\n✅ After-sales support & training\n\n🎯 Applications:\nAssembly | Welding | Pick & Place | Inspection | Palletizing\n\n📞 Contact us for a free consultation and custom quote.",
		"content_robotics_image":     "https://images.unsplash.com/photo-1485827404703-89b55fcc595e?w=800",
		"content_scada_body":         "🖥️ SCADA & HMI Development\nVisualize and control your entire operation from one screen.\n\n✅ Complete SCADA system development\n✅ MC Works64 & industry-standard platforms\n✅ HMI design for intuitive process control\n✅ Real-time monitoring & alarming\n✅ Historical data logging & trending\n✅ Multi-site remote monitoring\n✅ Custom reporting & dashboards\n✅ Mobile access to your plant data\n\n📊 Result: Complete plant visibility in real-time\n\n📞 Contact us for a free consultation and custom quote.",
		"content_scada_image":        "https://images.unsplash.com/photo-1551288049-bebda4e38f71?w=800",
		"content_seo_body":           "📈 Digital Marketing & SEO\nDominating search results and driving high-intent traffic.\n\n✅ Data-Driven SEO Strategies\n✅ ROI-Focused Google Ads (PPC)\n✅ LinkedIn B2B Lead Generation\n✅ Social Media Brand Positioning\n✅ Content Authority Building\n✅ Advanced Analytics & Tracking\n\nWe don't just get traffic; we get paying customers.",
		"content_seo_image":          "https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=800",
		"content_softwaremenu_body":  "💻 Software Solutions\nCustom-engineered software systems to automate and scale your business operations.\n\nSelect a specialized solution:",
		"content_softwaremenu_image": "https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=800",
		"content_solutions_body":     "🔧 Our Solutions — Ground to Cloud Automation\nFrom sensor-level data to cloud intelligence, we engineer the future of manufacturing.\n\n✅ PLC & SCADA Systems\n✅ Industrial Networking & IIoT\n✅ Digital Transformation (Software/ERP)\n✅ ATEX Certified Industrial Products\n✅ AI-Powered Data Analytics\n\nWhat are you looking for?",
		"content_solutions_image":    "https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=800",
		"content_webapp_body":        "🌐 Web & Mobile Portfolio\nEnterprise-grade digital products designed for high performance.\n\n✅ Progressive Web Apps (PWA)\n✅ High-Speed Corporate Websites\n✅ Mobile Apps (Flutter, React Native)\n✅ Headless CMS Solutions\n✅ Serverless API Architecture\n✅ AWS/GCP Cloud Deployment\n\nBuilt for speed, security, and extreme scalability. 🚀",
		"content_webapp_image":       "https://images.unsplash.com/photo-1555066931-4365d14bab8c?w=800",
		"content_whatsappbot_body":   "📱 WhatsApp Business Automation\nTransform your Customer Experience with 24/7 Intelligent Automation.\n\n✅ AI-Powered Conversation Flows\n✅ Full CRM & Database Integration\n✅ Automated Lead Qualification\n✅ Order Tracking & Payments\n✅ Multi-agent Admin Dashboard\n✅ Direct Broadcast Management\n\nScale your sales and support without adding headcount.",
		"content_whatsappbot_image":  "https://images.unsplash.com/photo-1611746872915-64382b5c76da?w=800",

		// ── Button labels ─────────────────────────────────────────────────
		// One label per action id. Written inline at 85 call sites before
		// this, which is how six actions ended up with two or three labels
		// each — a customer saw the same destination named differently
		// depending on the screen they came from.
		// WhatsApp truncates a button title at 20 characters.
		"btn_back_to_software":   "🔙 Software Menu",
		"btn_back_to_solutions":  "🔙 Back to Solutions",
		"btn_callback":           "2️⃣ Book a Callback",
		"btn_cat_app_dev":        "💻 Digital & Software",
		"btn_cat_automation":     "⚙️ Industrial Auto",
		"btn_cat_marketing":      "📈 Digital Marketing",
		"btn_cloud_analytics":    "☁️ Cloud Analytics",
		"btn_digital_software":   "💻 Digital & Software",
		"btn_expert":             "Talk to Expert 📞",
		"btn_flow_callback":      "📞 Book Callback",
		"btn_flow_quotation":     "💰 Get a Quote",
		"btn_flow_service":       "🔧 Service Request",
		"btn_general":            "💬 General Inquiry",
		"btn_get_free_quote":     "💬 Get Free Quote",
		"btn_iiot_analytics":     "📊 IIoT & Analytics",
		"btn_iiot_gateway":       "🌐 IIoT Gateway",
		"btn_industrial_auto":    "⚙️ Industrial Auto",
		"btn_industrial_sw":      "⚙️ Industrial Software",
		"btn_leave_casual":       "🛋️ CASUAL",
		"btn_leave_emergency":    "🚨 EMERGENCY",
		"btn_leave_sick":         "🤒 SICK LEAVE",
		"btn_main_menu":          "🏠 Main Menu",
		"btn_menu":               "Main Menu 🏠",
		"btn_next_to_analytics":  "⏭️ Cloud Analytics",
		"btn_next_to_panels":     "⏭️ Control Panels",
		"btn_next_to_robotics":   "⏭️ Next Service",
		"btn_next_to_scada":      "⏭️ Next Service",
		"btn_opt_out":            "🛑 Stop Messages",
		"btn_our_industries":     "🏭 Our Industries",
		"btn_our_solutions":      "🔧 Our Solutions",
		"btn_plc_control":        "⚡ PLC & Control",
		"btn_product":            "🛠️ Product Query",
		"btn_quotation":          "💰 Get a Quote",
		"btn_robotics":           "🤖 Robotics",
		"btn_scada_hmi":          "🖥️ SCADA & HMI",
		"btn_seo_marketing":      "📈 SEO & Marketing",
		"btn_service":            "🔧 Service Request",
		"btn_software_solutions": "💻 Software Solutions",
		"btn_talk_to_expert":     "📞 Talk to Expert",
		"btn_technical":          "🛠️ Technical Query",
		"btn_web_app_dev":        "🌐 Web & App Dev",
		"btn_whatsapp_bot":       "📱 WhatsApp Bots",

		"btn_start_day":            "🏢 START DAY",
		"btn_end_day":              "🏢 END DAY",
		"btn_apply_leave":          "🏝️ APPLY LEAVE",
		"content_welcome_body":     "🏭 Welcome to {{company}}!\nGround to Cloud — Engineering Smart Automation\n\nWe help manufacturers move from shop-floor control to cloud-connected intelligence.\n\n🎯 Trusted by industries across India\n⚡ 24/7 Automation Support\n🌐 www.askworx.in\n\nHow can we assist you today?",
		"content_welcome_image":    "https://images.unsplash.com/photo-1581091226825-a6a2a5aee158?w=800",
		"content_industrial_body":  "⚙️ Industrial Automation\nThe Foundation: Total Control at Machine Level\n\nWe design, build, and commission high-reliability automation systems engineered for continuous 24/7 industrial operations. Our experts specialize in creating seamless machine-level interfaces that maximize uptime.\n\n🔹 End-to-End System Design\n🔹 Retrofitting & Upgrades\n🔹 Machine Monitoring\n🔹 High-Performance Algorithms\n🔹 Safety-First Engineering\n\nSelect a service to learn more:",
		"content_industrial_image": "https://images.unsplash.com/photo-1537462715879-360eeb61a0ad?w=800",
		"content_software_body":    "💻 Digital & Software Solutions\nBridging traditional automation with modern digital thinking to scale your business.\n\nFrom automated customer engagement to full-scale enterprise software, we engineer tools that drive growth.",
		"content_software_image":   "https://images.unsplash.com/photo-1504868584819-f8e8b4b6d7e3?w=800",
		"content_iiot_body":        "📊 IIoT & Analytics Solutions\nConnecting your plant to the cloud.\n\nSelect a service:",
		"content_iiot_image":       "https://images.unsplash.com/photo-1518770660439-4636190af475?w=800",
	}

	for k, v := range defaults {
		Pool.Exec(context.Background(), "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING", k, v)
	}

	seedFAQs()
}

func seedFAQs() {
	faqs := []struct {
		keywords string
		answer   string
	}{
		{
			keywords: "what do you do, what is askworx, who are you, about askworx",
			answer:   fmt.Sprintf("%s provides end-to-end industrial automation — PLC & SCADA engineering, IIoT, cloud analytics, robotics, and digital software. 🏭\n\n🌐 www.askworx.in | 📞 +%s", os.Getenv("COMPANY_NAME"), os.Getenv("ADMIN_PHONE")),
		},
		{
			keywords: "plc, programmable logic controller",
			answer:   "We design & commission complete PLC systems — Micro PLCs, Motion Controllers, VFDs, Safety PLCs, and AC Servo Drives for Automotive, Pharma, Food & Beverage, and more. ⚡",
		},
		{
			keywords: "scada, hmi, supervisory control",
			answer:   "We develop full SCADA & HMI systems with real-time monitoring, alarming, historical trending, and multi-site remote access using platforms like MC Works64. 🖥️",
		},
		{
			keywords: "robot, robotics, cobot, collaborative robot",
			answer:   "We integrate industrial robots and cobots for welding, assembly, pick & place, and palletizing with full commissioning & after-sales support. 🤖",
		},
		{
			keywords: "iiot, industrial iot, iot gateway, mqtt, opc-ua, modbus",
			answer:   "Our IIoT solutions connect your machines to the cloud via OPC-UA, Modbus, and MQTT with secure edge computing and real-time dashboards. ☁️",
		},
		{
			keywords: "atex, hazardous area, explosion proof",
			answer:   fmt.Sprintf("We supply and support ATEX-certified instruments for hazardous area classifications. Contact our team for specific product recommendations. 🔒\n\n📞 +%s", os.Getenv("ADMIN_PHONE")),
		},
		{
			keywords: "price, pricing, cost, how much, quote, quotation",
			answer:   "Our pricing is tailored to your project scope. We deliver a detailed proposal within 24 hours. 💬",
		},
		{
			keywords: "contact, phone number, email, address, office, location",
			answer:   fmt.Sprintf("📞 +%s\n📧 contact@askworx.in\n🌐 www.askworx.in\n📍 1381, 6th Main Road, RR Nagar, Bangalore — 560098", os.Getenv("ADMIN_PHONE")),
		},
		{
			keywords: "whatsapp bot, chatbot, wa bot, automation bot",
			answer:   "We build AI-powered WhatsApp Business bots with CRM integration, lead capture, broadcast scheduling, and a multi-agent admin dashboard. 📱",
		},
		{
			keywords: "website, web app, mobile app, app development, flutter, react",
			answer:   "We build Progressive Web Apps, mobile apps (Flutter/React Native), corporate sites, and enterprise ERP/MES software. 🌐",
		},
	}

	for _, f := range faqs {
		Pool.Exec(context.Background(), "INSERT INTO faqs (keywords, answer) SELECT $1, $2 WHERE NOT EXISTS (SELECT 1 FROM faqs WHERE answer = $2)", f.keywords, f.answer)
	}
}

type FAQ struct {
	ID       int    `json:"id"`
	Keywords string `json:"keywords"`
	Answer   string `json:"answer"`
}

func GetAllFAQs() ([]FAQ, error) {
	rows, err := Pool.Query(context.Background(), "SELECT id, keywords, answer FROM faqs ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var faqs []FAQ
	for rows.Next() {
		var f FAQ
		rows.Scan(&f.ID, &f.Keywords, &f.Answer)
		faqs = append(faqs, f)
	}
	return faqs, nil
}

func SaveFAQ(f FAQ) error {
	if f.ID == 0 {
		_, err := Pool.Exec(context.Background(), "INSERT INTO faqs (keywords, answer) VALUES ($1, $2)", f.Keywords, f.Answer)
		return err
	}
	_, err := Pool.Exec(context.Background(), "UPDATE faqs SET keywords = $1, answer = $2 WHERE id = $3", f.Keywords, f.Answer, f.ID)
	return err
}

func DeleteFAQ(id int) error {
	_, err := Pool.Exec(context.Background(), "DELETE FROM faqs WHERE id = $1", id)
	return err
}

func GetSetting(key string) string {
	var val string
	err := Pool.QueryRow(context.Background(), "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func UpdateSetting(key, value string) error {
	_, err := Pool.Exec(context.Background(), "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value", key, value)
	return err
}

func GetAllSettings() (map[string]string, error) {
	rows, err := Pool.Query(context.Background(), "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		res[k] = v
	}
	return res, nil
}

func AddEmployee(name, phone string) error {
	_, err := Pool.Exec(context.Background(), `
		INSERT INTO employees (name, phone) VALUES ($1, $2)
		ON CONFLICT (phone) DO UPDATE SET name = EXCLUDED.name
	`, name, phone)
	return err
}

func IsEmployee(phone string) (bool, error) {
	var exists bool
	// Match last 10 digits to be safe with country codes
	err := Pool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM employees WHERE RIGHT(phone, 10) = RIGHT($1, 10))
	`, phone).Scan(&exists)
	return exists, err
}

func DeleteEmployee(id int) error {
	_, err := Pool.Exec(context.Background(), `DELETE FROM employees WHERE id = $1`, id)
	return err
}

type Employee struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

func GetEmployeesPaginated(limit, offset int) ([]Employee, error) {
	rows, err := Pool.Query(context.Background(), `SELECT id, name, phone, role FROM employees ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.Name, &e.Phone, &e.Role); err != nil {
			return nil, err
		}
		employees = append(employees, e)
	}
	return employees, nil
}

func GetTotalEmployeesCount() (int, error) {
	var count int
	err := Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM employees`).Scan(&count)
	return count, err
}

type AttendanceRecord struct {
	ID           int        `json:"id"`
	EmployeeName string     `json:"employee_name"`
	Date         time.Time  `json:"date"`
	CheckIn      *time.Time `json:"check_in"`
	CheckOut     *time.Time `json:"check_out"`
	WorkPlan     *string    `json:"work_plan"`
	EODReport    *string    `json:"eod_report"`
	CheckInLat   *float64   `json:"check_in_lat"`
	CheckInLng   *float64   `json:"check_in_lng"`
	CheckOutLat  *float64   `json:"check_out_lat"`
	CheckOutLng  *float64   `json:"check_out_lng"`
}

func GetAllEmployees() ([]Employee, error) {
	rows, err := Pool.Query(context.Background(), `SELECT id, name, phone, role FROM employees ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.Name, &e.Phone, &e.Role); err != nil {
			return nil, err
		}
		employees = append(employees, e)
	}
	return employees, nil
}

// GetEmployeesInServiceWindow returns employees who have messaged the bot in
// the last 24 hours — the only ones Meta will deliver the 9 AM greeting to,
// since it is not a template. See GetPhonesInServiceWindow.
func GetEmployeesInServiceWindow() ([]Employee, error) {
	rows, err := Pool.Query(context.Background(), `
		SELECT e.id, e.name, e.phone, e.role
		FROM employees e
		WHERE EXISTS (
			SELECT 1 FROM messages_log m
			WHERE RIGHT(m.phone, 10) = RIGHT(e.phone, 10)
			  AND m.direction = 'incoming'
			  AND m.sent_at > NOW() - INTERVAL '24 hours')
		ORDER BY e.name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.Name, &e.Phone, &e.Role); err != nil {
			return nil, err
		}
		employees = append(employees, e)
	}
	return employees, rows.Err()
}

func GetAttendancePaginated(limit, offset int, start, end string) ([]AttendanceRecord, error) {
	query := `
		SELECT a.id, COALESCE(e.name, 'Unregistered Staff'), a.date, a.check_in, a.check_out, a.work_plan, a.eod_report,
		       a.check_in_lat, a.check_in_lng, a.check_out_lat, a.check_out_lng
		FROM attendance a
		LEFT JOIN employees e ON RIGHT(a.phone, 10) = RIGHT(e.phone, 10)
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND a.date >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND a.date <= $%d", argID)
		args = append(args, end)
		argID++
	}

	query += fmt.Sprintf(" ORDER BY a.date DESC, a.check_in DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AttendanceRecord
	for rows.Next() {
		var r AttendanceRecord
		if err := rows.Scan(&r.ID, &r.EmployeeName, &r.Date, &r.CheckIn, &r.CheckOut, &r.WorkPlan, &r.EODReport,
			&r.CheckInLat, &r.CheckInLng, &r.CheckOutLat, &r.CheckOutLng); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func GetTotalAttendanceCount(start, end string) (int, error) {
	query := `SELECT COUNT(*) FROM attendance WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND date >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND date <= $%d", argID)
		args = append(args, end)
		argID++
	}

	var count int
	err := Pool.QueryRow(context.Background(), query, args...).Scan(&count)
	return count, err
}

type LeaveRequest struct {
	ID            int    `json:"id"`
	EmployeeName  string `json:"employee_name"`
	EmployeePhone string `json:"employee_phone"`
	LeaveType     string `json:"leave_type"`
	LeaveDate     string `json:"leave_date"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
}

func GetLeaveRequestsPaginated(limit, offset int, start, end string) ([]LeaveRequest, error) {
	query := `
		SELECT l.id, COALESCE(e.name, 'Unregistered Staff'), l.phone, l.leave_type, l.leave_date, l.reason, l.status
		FROM leave_requests l
		LEFT JOIN employees e ON RIGHT(l.phone, 10) = RIGHT(e.phone, 10)
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND l.leave_date >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND l.leave_date <= $%d", argID)
		args = append(args, end)
		argID++
	}

	query += fmt.Sprintf(" ORDER BY l.id DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []LeaveRequest
	for rows.Next() {
		var r LeaveRequest
		if err := rows.Scan(&r.ID, &r.EmployeeName, &r.EmployeePhone, &r.LeaveType, &r.LeaveDate, &r.Reason, &r.Status); err != nil {
			return nil, err
		}
		requests = append(requests, r)
	}
	return requests, nil
}

func GetTotalLeaveRequestsCount(start, end string) (int, error) {
	query := `SELECT COUNT(*) FROM leave_requests WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND leave_date >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND leave_date <= $%d", argID)
		args = append(args, end)
		argID++
	}

	var count int
	err := Pool.QueryRow(context.Background(), query, args...).Scan(&count)
	return count, err
}

func UpdateLeaveStatus(id int, status string) (string, error) {
	var phone string
	err := Pool.QueryRow(context.Background(), `
		UPDATE leave_requests SET status = $1 WHERE id = $2
		RETURNING phone
	`, status, id).Scan(&phone)
	return phone, err
}

func CreateReminder(phone, desc string, due time.Time) error {
	_, err := Pool.Exec(context.Background(), `
		INSERT INTO reminders (employee_phone, description, due_at, status) VALUES ($1, $2, $3, 'scheduled')
	`, phone, desc, due)
	return err
}

func GetDueReminders() ([]struct {
	ID    int
	Phone string
	Desc  string
}, error) {
	rows, err := Pool.Query(context.Background(), `
		SELECT id, employee_phone, description FROM reminders 
		WHERE status = 'scheduled' AND due_at <= $1
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []struct {
		ID    int
		Phone string
		Desc  string
	}
	for rows.Next() {
		var r struct {
			ID    int
			Phone string
			Desc  string
		}
		if err := rows.Scan(&r.ID, &r.Phone, &r.Desc); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, nil
}

func MarkReminderSent(id int) error {
	_, err := Pool.Exec(context.Background(), `UPDATE reminders SET status = 'sent' WHERE id = $1`, id)
	return err
}

type ReminderRecord struct {
	ID     int       `json:"id"`
	Phone  string    `json:"phone"`
	Name   string    `json:"name"`
	Desc   string    `json:"description"`
	DueAt  time.Time `json:"due_at"`
	Status string    `json:"status"`
}

func GetReminders(limit, offset int, start, end string) ([]ReminderRecord, error) {
	query := `
		SELECT r.id, r.employee_phone, COALESCE(e.name, 'Staff'), r.description, r.due_at, r.status
		FROM reminders r
		LEFT JOIN employees e ON r.employee_phone = e.phone
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND r.due_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND r.due_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	query += fmt.Sprintf(" ORDER BY r.due_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []ReminderRecord
	for rows.Next() {
		var r ReminderRecord
		if err := rows.Scan(&r.ID, &r.Phone, &r.Name, &r.Desc, &r.DueAt, &r.Status); err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	return reminders, nil
}

func GetTotalRemindersCount(start, end string) (int, error) {
	query := `SELECT COUNT(*) FROM reminders WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND due_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND due_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	var count int
	err := Pool.QueryRow(context.Background(), query, args...).Scan(&count)
	return count, err
}

func HasCheckedInToday(phone string) (bool, error) {
	var exists bool
	err := Pool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM attendance WHERE phone = $1 AND date = CURRENT_DATE AND check_in IS NOT NULL)
	`, phone).Scan(&exists)
	return exists, err
}

func HasCheckedOutToday(phone string) (bool, error) {
	var exists bool
	err := Pool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM attendance WHERE phone = $1 AND date = CURRENT_DATE AND check_out IS NOT NULL)
	`, phone).Scan(&exists)
	return exists, err
}

func MarkCheckIn(phone string, lat, lng float64) error {
	_, err := Pool.Exec(context.Background(), `
		INSERT INTO attendance (phone, check_in, check_in_lat, check_in_lng) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (phone, date) DO UPDATE 
		SET check_in = EXCLUDED.check_in,
		    check_in_lat = EXCLUDED.check_in_lat,
		    check_in_lng = EXCLUDED.check_in_lng
		WHERE attendance.check_in IS NULL
	`, phone, time.Now(), lat, lng)
	return err
}

func UpdateWorkPlan(phone, plan string) error {
	_, err := Pool.Exec(context.Background(), `
		UPDATE attendance SET work_plan = $1 
		WHERE phone = $2 AND date = CURRENT_DATE
	`, plan, phone)
	return err
}

func MarkCheckOut(phone string, lat, lng float64) error {
	_, err := Pool.Exec(context.Background(), `
		UPDATE attendance 
		SET check_out = $1, check_out_lat = $2, check_out_lng = $3 
		WHERE phone = $4 AND date = CURRENT_DATE
	`, time.Now(), lat, lng, phone)
	return err
}

func UpdateEODReport(phone, report string) error {
	_, err := Pool.Exec(context.Background(), `
		UPDATE attendance SET eod_report = $1 
		WHERE phone = $2 AND date = CURRENT_DATE
	`, report, phone)
	return err
}

func SubmitLeave(phone, lType, date, reason string) error {
	_, err := Pool.Exec(context.Background(), `
		INSERT INTO leave_requests (phone, leave_type, leave_date, reason) 
		VALUES ($1, $2, $3, $4)
	`, phone, lType, date, reason)
	return err
}

// CreateAnnouncementRecord stores a broadcast in the reminders table as 'sent' for history tracking
func CreateAnnouncementRecord(phone, message string, sentAt time.Time) error {
	_, err := Pool.Exec(context.Background(), `
		INSERT INTO reminders (employee_phone, description, due_at, status) VALUES ($1, $2, $3, 'sent')
	`, phone, message, sentAt)
	return err
}

// SettingOr returns the stored value for key, falling back to def when the
// row is missing or has been emptied.
//
// GetSetting returns "" both when a key does not exist and when its value is
// genuinely blank, so calling it directly meant an unseeded key sent an empty
// WhatsApp message. Every configurable message goes through here instead, so
// the shipped wording is always the floor.
func SettingOr(key, def string) string {
	if v := GetSetting(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// GetEmployeeName returns the stored name for a phone number, or "" if the
// number is not on the team. Used so a message template can greet somebody by
// name instead of by handset.
func GetEmployeeName(phone string) string {
	var name string
	err := Pool.QueryRow(context.Background(),
		"SELECT name FROM employees WHERE RIGHT(phone, 10) = RIGHT($1, 10)", phone).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

// ButtonLabel returns the label for a WhatsApp button, keyed by its action id.
//
// Labels were written inline at every call site, so the same action carried
// different words depending on which screen you arrived from — `main_menu` was
// "🏠 Main Menu", "Main Menu 🏠" and "🏠 Explore Menu" in different places, and
// `cat_marketing` was labelled "📊 IIoT & Analytics" on one screen, which is a
// different service entirely. Routing every label through one key per id makes
// the label a property of the action rather than of the screen.
//
// WhatsApp truncates a button title at 20 characters, so an over-long label is
// clipped rather than sent — the admin panel shows the count.
func ButtonLabel(id, def string) string {
	return SettingOr("btn_"+id, def)
}
