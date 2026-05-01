package db

import (
	"context"
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
			continue
		}
		phones = append(phones, p)
	}
	return phones, nil
}
