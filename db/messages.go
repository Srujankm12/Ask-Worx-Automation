package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func LogMessage(phone, direction, message string, waID ...string) error {
	msgID := ""
	if len(waID) > 0 {
		msgID = waID[0]
	}

	if msgID != "" {
		_, err := Pool.Exec(context.Background(),
			"INSERT INTO messages_log (phone, direction, message, wa_msg_id) VALUES ($1, $2, $3, $4) ON CONFLICT (wa_msg_id) DO NOTHING",
			phone, direction, message, msgID)
		return err
	}

	_, err := Pool.Exec(context.Background(),
		"INSERT INTO messages_log (phone, direction, message) VALUES ($1, $2, $3)",
		phone, direction, message)
	return err
}

// LogIncomingMessage records an inbound message and reports whether this call
// is the one that actually stored it.
//
// Meta delivers every webhook at least once, and in practice sends each
// message twice — two POSTs in the same second, carrying the same wa_msg_id.
// The unique index deduplicated the log, so the conversation looked correct,
// but webhook.go handed both copies to handleMessage and the bot answered
// every customer twice.
//
// The insert is the lock: exactly one caller sees a row affected, and only
// that caller should act on the message. A message with no id cannot be
// deduplicated, so it is processed — answering twice is better than staying
// silent.
func LogIncomingMessage(phone, message, waID string) (bool, error) {
	if waID == "" {
		return true, LogMessage(phone, "incoming", message)
	}

	tag, err := Pool.Exec(context.Background(),
		`INSERT INTO messages_log (phone, direction, message, wa_msg_id)
		 VALUES ($1, 'incoming', $2, $3)
		 ON CONFLICT (wa_msg_id) DO NOTHING`,
		phone, message, waID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Alias for consistency
func SaveMessageHistory(phone, message, direction string) error {
	return LogMessage(phone, direction, message)
}

type MessageLog struct {
	ID        int       `json:"id"`
	Phone     string    `json:"phone"`
	Direction string    `json:"direction"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
}

func GetAllMessages() ([]MessageLog, error) {
	rows, err := Pool.Query(context.Background(), "SELECT id, phone, direction, message, sent_at FROM messages_log ORDER BY sent_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []MessageLog{}
	for rows.Next() {
		var l MessageLog
		err := rows.Scan(&l.ID, &l.Phone, &l.Direction, &l.Message, &l.SentAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/messages.go", err)
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func GetMessagesByPhone(phone string) ([]MessageLog, error) {
	rows, err := Pool.Query(context.Background(), "SELECT id, phone, direction, message, sent_at FROM messages_log WHERE phone = $1 ORDER BY sent_at ASC", phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []MessageLog{}
	for rows.Next() {
		var l MessageLog
		err := rows.Scan(&l.ID, &l.Phone, &l.Direction, &l.Message, &l.SentAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/messages.go", err)
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func GetAllPhoneNumbers() ([]string, error) {
	rows, err := Pool.Query(context.Background(), "SELECT DISTINCT phone FROM contacts WHERE opt_out = FALSE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phones := []string{}
	for rows.Next() {
		var p string
		err := rows.Scan(&p)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/messages.go", err)
			continue
		}
		phones = append(phones, p)
	}
	return phones, nil
}

// GetPhonesInServiceWindow returns opted-in contacts who have messaged the bot
// in the last 24 hours. WhatsApp only accepts a non-template message inside
// that window; sending to anyone else is rejected by Meta (error 131047), so
// the scheduled greeting and campaigns use this list instead of
// GetAllPhoneNumbers.
//
// Phones are compared on their last ten digits, as the attendance query does,
// because contacts added from the panel may not carry the country code.
func GetPhonesInServiceWindow() ([]string, error) {
	rows, err := Pool.Query(context.Background(), `
		SELECT DISTINCT c.phone
		FROM contacts c
		JOIN messages_log m ON RIGHT(m.phone, 10) = RIGHT(c.phone, 10)
		WHERE c.opt_out = FALSE
		  AND m.direction = 'incoming'
		  AND m.sent_at > NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phones := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/messages.go", err)
			continue
		}
		phones = append(phones, p)
	}
	return phones, rows.Err()
}

// IsInServiceWindow reports whether phone has messaged the bot in the last 24
// hours, which is when a free-form reply can still be delivered.
func IsInServiceWindow(phone string) (bool, error) {
	var in bool
	err := Pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM messages_log
			WHERE RIGHT(phone, 10) = RIGHT($1, 10)
			  AND direction = 'incoming'
			  AND sent_at > NOW() - INTERVAL '24 hours')`, phone).Scan(&in)
	return in, err
}

// ── Aggregates, computed in the database ────────────────────────────────────
//
// The dashboard used to pull the message log over the wire and bucket it in the
// browser. GetAllMessages has no LIMIT, so "the message log" meant every row
// ever written, re-fetched every thirty seconds by every open tab, to draw two
// small charts. These two queries return a few dozen rows instead.

// DayCount is one day of conversation volume.
type DayCount struct {
	Day      string `json:"day"` // YYYY-MM-DD in the business time zone
	Incoming int    `json:"incoming"`
	Outgoing int    `json:"outgoing"`
}

// HourCount is one hour of the 24-hour clock, summed across the window.
type HourCount struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

// MessageSummary is what the dashboard needs and nothing more.
type MessageSummary struct {
	Days     int         `json:"days"`
	TimeZone string      `json:"time_zone"`
	Daily    []DayCount  `json:"daily"`
	Hourly   []HourCount `json:"hourly"`
	Total    int         `json:"total"`
}

// BusinessTimeZone is the zone days and hours are bucketed in. It matches the
// zone the scheduler fires in, so "busiest around 3pm" means the same thing on
// the dashboard as it does to whoever is staffing the replies.
func BusinessTimeZone() string {
	if tz := os.Getenv("TIMEZONE"); tz != "" {
		return tz
	}
	return "Asia/Kolkata"
}

// GetMessageSummary returns per-day and per-hour counts over the last n days.
func GetMessageSummary(days int) (MessageSummary, error) {
	if days < 1 {
		days = 14
	}
	if days > 365 {
		days = 365
	}

	tz := BusinessTimeZone()
	summary := MessageSummary{
		Days:     days,
		TimeZone: tz,
		Daily:    []DayCount{},
		Hourly:   []HourCount{},
	}

	// One window shared by both queries: midnight, days-1 ago, local time.
	window := fmt.Sprintf("(date_trunc('day', NOW() AT TIME ZONE '%s') - INTERVAL '%d days')", tz, days-1)

	dailySQL := fmt.Sprintf(`
		SELECT to_char(date_trunc('day', sent_at AT TIME ZONE '%s'), 'YYYY-MM-DD') AS day,
		       COUNT(*) FILTER (WHERE direction = 'incoming') AS incoming,
		       COUNT(*) FILTER (WHERE direction <> 'incoming') AS outgoing
		FROM messages_log
		WHERE sent_at AT TIME ZONE '%s' >= %s
		GROUP BY 1
		ORDER BY 1`, tz, tz, window)

	rows, err := Pool.Query(context.Background(), dailySQL)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Day, &d.Incoming, &d.Outgoing); err != nil {
			rows.Close()
			return summary, err
		}
		summary.Daily = append(summary.Daily, d)
		summary.Total += d.Incoming + d.Outgoing
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return summary, err
	}

	hourlySQL := fmt.Sprintf(`
		SELECT EXTRACT(HOUR FROM sent_at AT TIME ZONE '%s')::int AS hour, COUNT(*)
		FROM messages_log
		WHERE sent_at AT TIME ZONE '%s' >= %s
		GROUP BY 1
		ORDER BY 1`, tz, tz, window)

	hrows, err := Pool.Query(context.Background(), hourlySQL)
	if err != nil {
		return summary, err
	}
	defer hrows.Close()
	for hrows.Next() {
		var h HourCount
		if err := hrows.Scan(&h.Hour, &h.Count); err != nil {
			return summary, err
		}
		summary.Hourly = append(summary.Hourly, h)
	}
	return summary, hrows.Err()
}

// GetMessagesByPhoneAfter returns only the messages newer than afterID.
//
// The inbox polls this every few seconds; refetching the whole conversation
// each time meant a full scan of messages_log per poll, per open tab.
func GetMessagesByPhoneAfter(phone string, afterID int) ([]MessageLog, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, phone, direction, message, sent_at
		 FROM messages_log
		 WHERE phone = $1 AND id > $2
		 ORDER BY id ASC`, phone, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []MessageLog{}
	for rows.Next() {
		var l MessageLog
		if err := rows.Scan(&l.ID, &l.Phone, &l.Direction, &l.Message, &l.SentAt); err != nil {
			log.Printf("[db] db/messages.go: skipping unreadable row: %v", err)
			continue
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetMessagesPage returns a bounded page of the whole log, newest first.
// GetAllMessages is kept for callers that genuinely want everything, but no
// HTTP handler should use it.
func GetMessagesPage(limit, offset int) ([]MessageLog, int, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM messages_log`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := Pool.Query(context.Background(),
		`SELECT id, phone, direction, message, sent_at
		 FROM messages_log ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := []MessageLog{}
	for rows.Next() {
		var l MessageLog
		if err := rows.Scan(&l.ID, &l.Phone, &l.Direction, &l.Message, &l.SentAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}
