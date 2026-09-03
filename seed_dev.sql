-- ============================================================================
-- LOCAL DEVELOPMENT FIXTURES — NOT FOR ANY DEPLOYED DATABASE
--
-- Every person, company and phone number below is invented. The numbers use
-- the prefix 91 1000 00xx xx: +91 is India, but a real Indian mobile never
-- begins with a 1, so none of these can route to an actual handset. That is
-- deliberate — seed data that dials a stranger is not seed data.
--
-- Run:    psql -d askworx -f seed_dev.sql
-- Reset:  psql -d askworx -f seed_dev.sql   (it truncates first, so it is
--                                            safe to run repeatedly)
-- ============================================================================

BEGIN;

TRUNCATE leads, contacts, employees, callbacks, attendance,
         leave_requests, reminders, campaigns, messages_log, faqs
  RESTART IDENTITY CASCADE;

-- ── Contacts ────────────────────────────────────────────────────────────────
INSERT INTO contacts (phone, name, company, joined_at) VALUES
  ('911000001101', 'Rajesh Menon',      'Northline Polymers',        now() - interval '62 days'),
  ('911000001102', 'Priya Sundaram',    'Vantage Extrusions',        now() - interval '55 days'),
  ('911000001103', 'Imran Qureshi',     'Sixth Axis Robotics',       now() - interval '41 days'),
  ('911000001104', 'Anita Deshpande',   'Halcyon Pharma Systems',    now() - interval '33 days'),
  ('911000001105', 'Vikram Rao',        'Deccan Bottling Works',     now() - interval '28 days'),
  ('911000001106', 'Fatima Sheikh',     'Arclight Fabrication',      now() - interval '19 days'),
  ('911000001107', 'Suresh Iyer',       'Kaveri Cold Chain',         now() - interval '12 days'),
  ('911000001108', 'Meera Krishnan',    'Bluestone Packaging',       now() - interval '9 days'),
  ('911000001109', 'Arjun Nair',        NULL,                        now() - interval '5 days'),
  ('911000001110', 'Divya Prakash',     'Trident Auto Components',   now() - interval '2 days'),
  ('911000001111', 'Karthik Reddy',     'Solaris Battery Systems',   now() - interval '1 day'),
  ('911000001112', 'Neha Bhatt',        'Vertex Precision Tools',    now() - interval '6 hours');

-- ── Leads ───────────────────────────────────────────────────────────────────
-- Spread across all four statuses and both enquiry types, so the filters,
-- badges, status buttons and pagination all have something to show.
INSERT INTO leads (phone, name, company, requirement, contact_phone, status, created_at) VALUES
  ('911000001112', 'Neha Bhatt',      'Vertex Precision Tools',   'Quotation: retrofit 3 CNC cells with VFD drives and a shared HMI', '911000001112', 'new',         now() - interval '3 hours'),
  ('911000001111', 'Karthik Reddy',   'Solaris Battery Systems',  'Request: SCADA for a new 40 MWh cell assembly line, needs OPC UA to existing MES', '911000001111', 'new',        now() - interval '9 hours'),
  ('911000001110', 'Divya Prakash',   'Trident Auto Components',  'Looking for pick-and-place cobots for a press shop, 6 stations', '911000001110', 'new',          now() - interval '1 day'),
  ('911000001109', 'Arjun Nair',      NULL,                       'General enquiry about IIoT dashboards, no timeline yet', NULL,                     'new',          now() - interval '1 day 4 hours'),
  ('911000001108', 'Meera Krishnan',  'Bluestone Packaging',      'Query: energy monitoring across 4 shop floors, wants per-machine kWh', '911000001108', 'called',   now() - interval '2 days'),
  ('911000001107', 'Suresh Iyer',     'Kaveri Cold Chain',        'Quotation: cold-store temperature logging with SMS alerts, 22 chambers', '911000001107', 'called', now() - interval '3 days'),
  ('911000001106', 'Fatima Sheikh',   'Arclight Fabrication',     'Robotic welding cell integration, 2 stations, existing Fanuc arms', '911000001106', 'in_progress', now() - interval '5 days'),
  ('911000001105', 'Vikram Rao',      'Deccan Bottling Works',    'Request: line efficiency (OEE) reporting for 3 filling lines', '911000001105',     'in_progress', now() - interval '8 days'),
  ('911000001104', 'Anita Deshpande', 'Halcyon Pharma Systems',   'Quotation: 21 CFR Part 11 compliant batch record logging', '911000001104',        'in_progress', now() - interval '11 days'),
  ('911000001103', 'Imran Qureshi',   'Sixth Axis Robotics',      'Vision-guided inspection for an assembly line, 400 parts/hour', '911000001103',   'converted',   now() - interval '18 days'),
  ('911000001102', 'Priya Sundaram',  'Vantage Extrusions',       'Request: PLC migration from legacy relay logic, 2 extruders', '911000001102',      'converted',   now() - interval '26 days'),
  ('911000001101', 'Rajesh Menon',    'Northline Polymers',       'Quotation: plant-wide SCADA with cloud historian', '911000001101',                 'converted',   now() - interval '40 days');

-- ── Callbacks ───────────────────────────────────────────────────────────────
INSERT INTO callbacks (phone, name, preferred_time, status, created_at) VALUES
  ('911000001112', 'Neha Bhatt',      'Today, after 4 PM',           'pending', now() - interval '2 hours'),
  ('911000001111', 'Karthik Reddy',   'Tomorrow morning, 10-11 AM',  'pending', now() - interval '8 hours'),
  ('911000001108', 'Meera Krishnan',  'Any weekday after 6 PM',      'pending', now() - interval '1 day'),
  ('911000001106', 'Fatima Sheikh',   'Monday, 11 AM',               'done',    now() - interval '6 days'),
  ('911000001103', 'Imran Qureshi',   'Whenever convenient',         'done',    now() - interval '17 days');

-- ── Employees ───────────────────────────────────────────────────────────────
INSERT INTO employees (name, phone, role) VALUES
  ('Sandeep Kulkarni', '911000002201', 'Field Engineer'),
  ('Lakshmi Narayan',  '911000002202', 'Project Manager'),
  ('Aditya Verma',     '911000002203', 'Automation Engineer'),
  ('Ritu Chandra',     '911000002204', 'Sales Executive'),
  ('Mohammed Ashraf',  '911000002205', 'Commissioning Engineer'),
  ('Sneha Pillai',     '911000002206', 'SCADA Developer');

-- ── Attendance ──────────────────────────────────────────────────────────────
-- Today plus the two working days before it, with one open shift (no check-out
-- yet) and one absence, so the page shows more than a wall of green.
INSERT INTO attendance (phone, check_in, check_out, check_in_lat, check_in_lng, work_plan, eod_report, date) VALUES
  ('911000002201', now() - interval '7 hours', now() - interval '20 minutes', 12.9716, 77.5946, 'Site visit at Northline Polymers — commission the new VFD panel and hand over drawings.', 'Panel commissioned and signed off. Two spare relays short, raised a request with stores.', CURRENT_DATE),
  ('911000002203', now() - interval '8 hours', NULL, 12.9352, 77.6245, 'Finish OPC UA tag mapping for the Solaris pilot, then start the FAT checklist.', NULL, CURRENT_DATE),
  ('911000002206', now() - interval '8 hours', now() - interval '15 minutes', 12.9141, 77.6101, 'SCADA screens for Deccan Bottling — alarm pages and trend views.', 'Alarm pages done. Trends pending, blocked on historian credentials from client IT.', CURRENT_DATE),
  ('911000002202', now() - interval '1 day 8 hours', now() - interval '1 day', 12.9716, 77.5946, 'Project review with Halcyon, then update the Gantt for the pharma batch job.', 'Review done. Client moved the FAT date out by two weeks; schedule updated.', CURRENT_DATE - 1),
  ('911000002201', now() - interval '1 day 8 hours', now() - interval '1 day 1 hour', 13.0102, 77.5510, 'Panel wiring at Arclight, second welding station.', 'Wiring complete up to terminal 40. Resumes tomorrow.', CURRENT_DATE - 1),
  ('911000002204', now() - interval '2 days 8 hours', now() - interval '2 days', 12.9698, 77.7500, 'Follow up on three open quotations and log outcomes.', 'Two confirmed, one asked for revised pricing. Notes added against each lead.', CURRENT_DATE - 2);

-- ── Leave requests ──────────────────────────────────────────────────────────
INSERT INTO leave_requests (phone, leave_type, leave_date, reason, status) VALUES
  ('911000002204', 'Casual Leave',  CURRENT_DATE + 3, 'Family function out of town.',                     'pending'),
  ('911000002205', 'Sick Leave',    CURRENT_DATE + 1, 'Viral fever, doctor advised two days rest.',       'pending'),
  ('911000002202', 'Earned Leave',  CURRENT_DATE + 12, 'Planned holiday, cover arranged with Sneha.',     'pending'),
  ('911000002203', 'Casual Leave',  CURRENT_DATE - 4, 'House shifting.',                                  'approved'),
  ('911000002206', 'Sick Leave',    CURRENT_DATE - 9, 'Dental surgery.',                                  'approved'),
  ('911000002201', 'Casual Leave',  CURRENT_DATE - 2, 'Personal work.',                                   'rejected');

-- ── Reminders ───────────────────────────────────────────────────────────────
INSERT INTO reminders (employee_phone, description, due_at, status) VALUES
  ('911000002201', 'Collect the signed handover note from Northline Polymers.',        now() + interval '4 hours',  'pending'),
  ('911000002203', 'Send the FAT checklist to Solaris before end of day.',             now() + interval '6 hours',  'pending'),
  ('911000002204', 'Follow up on the revised quotation for Kaveri Cold Chain.',        now() + interval '1 day',    'pending'),
  ('911000002206', 'Chase historian credentials from Deccan Bottling IT.',             now() + interval '2 days',   'pending'),
  ('911000002202', 'Circulate the updated Halcyon project schedule.',                  now() - interval '1 day',    'sent'),
  ('911000002205', 'Submit commissioning report for the Arclight welding cell.',       now() - interval '3 days',   'sent');

-- ── Campaigns ───────────────────────────────────────────────────────────────
INSERT INTO campaigns (type, question, option_a, option_b, option_c, correct_answer, explanation, image_url, caption, scheduled_at, status, total_sent, created_at) VALUES
  ('quiz',   'Which protocol is designed for machine-to-machine communication over TCP/IP in industrial settings?', 'HTTP', 'OPC UA', 'FTP', 'B', 'OPC UA is platform-independent and carries both data and its meaning, which is why it has become the default for plant-to-cloud links.', '', '', '', now() + interval '2 days', 'scheduled', 0,  now() - interval '1 day'),
  ('quiz',   'What does a VFD primarily control in an induction motor?',                                            'Torque only', 'Speed and torque', 'Direction only', 'B', 'A variable frequency drive varies supply frequency and voltage together, so it controls both speed and available torque.', '', '', '', now() - interval '7 days', 'sent', 128, now() - interval '10 days'),
  ('poster', '', '', '', '', '', '', '', '/uploads/sample-poster.png', 'Ground to Cloud — our SCADA and IIoT capability, in one page.', now() - interval '14 days', 'sent', 214, now() - interval '16 days');

-- ── Message log ─────────────────────────────────────────────────────────────
INSERT INTO messages_log (phone, direction, message, sent_at) VALUES
  ('911000001112', 'incoming', 'Hi, do you handle VFD retrofits on older CNC machines?',                            now() - interval '3 hours 20 minutes'),
  ('911000001112', 'outgoing', 'Yes. Could you share the machine make and the drive rating you are running today?', now() - interval '3 hours 18 minutes'),
  ('911000001112', 'incoming', 'Three Mazak cells, 15 kW each. Panels are from 2009.',                              now() - interval '3 hours 11 minutes'),
  ('911000001112', 'outgoing', 'Noted. An engineer will call you today to walk through the retrofit scope.',        now() - interval '3 hours 9 minutes'),
  ('911000001111', 'incoming', 'Need SCADA for a battery line. Can you integrate with our existing MES?',           now() - interval '9 hours'),
  ('911000001111', 'outgoing', 'We can, over OPC UA. Which MES are you running?',                                   now() - interval '8 hours 55 minutes'),
  ('911000001111', 'incoming', 'SAP ME. Line is 40 MWh, commissioning in March.',                                   now() - interval '8 hours 40 minutes'),
  ('911000001108', 'incoming', 'What does per-machine energy monitoring usually cost?',                             now() - interval '2 days'),
  ('911000001108', 'outgoing', 'It depends on meter count and whether the panels have spare CT space. Roughly how many machines?', now() - interval '2 days' + interval '3 minutes'),
  ('911000001108', 'incoming', 'About 60 across four floors.',                                                      now() - interval '2 days' + interval '20 minutes'),
  ('911000001105', 'incoming', 'Can you show OEE per shift rather than per day?',                                   now() - interval '8 days'),
  ('911000001105', 'outgoing', 'Yes — shift boundaries are configurable, and downtime reasons can be captured at the machine.', now() - interval '8 days' + interval '6 minutes');

-- ── FAQ knowledge base ──────────────────────────────────────────────────────
INSERT INTO faqs (keywords, answer, created_at) VALUES
  ('price, cost, pricing, quotation, quote', 'Pricing depends on scope, so we quote per project rather than from a rate card. Share your requirement and an engineer will come back with a written proposal, usually within two working days.', now() - interval '30 days'),
  ('location, address, office, where',       'Our office is in Bangalore, and we take on commissioning work across India. Site visits are arranged once the scope is agreed.', now() - interval '30 days'),
  ('support, amc, maintenance, service',     'We offer annual maintenance contracts covering remote support and scheduled site visits. Response times are set in the contract — tell us your uptime requirement and we will size it.', now() - interval '25 days'),
  ('plc, scada, hmi, automation',            'We work across PLC, SCADA and HMI platforms, including migrations from legacy relay logic. Tell us the make and model you have and we will confirm what is involved.', now() - interval '22 days'),
  ('iiot, cloud, dashboard, monitoring',     'We connect plant equipment to cloud dashboards over OPC UA or MQTT, with on-premise historians where data cannot leave site.', now() - interval '15 days'),
  ('timeline, how long, delivery, duration', 'A typical retrofit runs four to eight weeks from purchase order to handover. Larger SCADA projects are scheduled against your shutdown window.', now() - interval '10 days');

COMMIT;

\echo ''
\echo 'Seeded. Row counts:'
SELECT 'leads' AS table, count(*) FROM leads
UNION ALL SELECT 'contacts',       count(*) FROM contacts
UNION ALL SELECT 'employees',      count(*) FROM employees
UNION ALL SELECT 'callbacks',      count(*) FROM callbacks
UNION ALL SELECT 'attendance',     count(*) FROM attendance
UNION ALL SELECT 'leave_requests', count(*) FROM leave_requests
UNION ALL SELECT 'reminders',      count(*) FROM reminders
UNION ALL SELECT 'campaigns',      count(*) FROM campaigns
UNION ALL SELECT 'messages_log',   count(*) FROM messages_log
UNION ALL SELECT 'faqs',           count(*) FROM faqs
ORDER BY 1;
