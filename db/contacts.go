package db

import (
	"context"
	"log"
	"time"
)

func SaveContact(phone string, name string) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO contacts (phone, name) VALUES ($1, $2) ON CONFLICT (phone) DO UPDATE SET name = EXCLUDED.name WHERE contacts.name IS NULL OR contacts.name = ''",
		phone, name)
	return err
}

func UpsertContact(phone, name string) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO contacts (phone, name) VALUES ($1, $2) ON CONFLICT (phone) DO UPDATE SET name = EXCLUDED.name",
		phone, name)
	return err
}

func UpdateContactName(phone, name string) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE contacts SET name = $1 WHERE phone = $2",
		name, phone)
	return err
}

func UpdateContactOptOut(id int, optOut bool) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE contacts SET opt_out = $1 WHERE id = $2",
		optOut, id)
	return err
}

func UpdateContactOptOutByPhone(phone string, optOut bool) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE contacts SET opt_out = $1 WHERE phone = $2",
		optOut, phone)
	return err
}

func DeleteContact(id int) error {
	_, err := Pool.Exec(context.Background(),
		"DELETE FROM contacts WHERE id = $1",
		id)
	return err
}

func UpdateContactCompany(phone, company string) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE contacts SET company = $1 WHERE phone = $2",
		company, phone)
	return err
}

type Contact struct {
	ID       int       `json:"id"`
	Phone    string    `json:"phone"`
	Name     *string   `json:"name"`
	Company  *string   `json:"company"`
	OptOut   bool      `json:"opt_out"`
	JoinedAt time.Time `json:"joined_at"`
	// When they last messaged the bot, or null if they never have. The panel
	// reads this to show who is inside WhatsApp's 24-hour reply window.
	LastIncomingAt *time.Time `json:"last_incoming_at"`
}

func SyncNamesFromLeads() error {
	_, err := Pool.Exec(context.Background(), `
		UPDATE contacts 
		SET name = leads.name, company = leads.company
		FROM leads 
		WHERE contacts.phone = leads.phone 
		AND (contacts.name IS NULL OR contacts.name = '')
	`)
	return err
}

func GetAllContacts() ([]Contact, error) {
	// Sync names before fetching
	SyncNamesFromLeads()

	query := `
		SELECT c.id, c.phone, c.name, c.company, c.opt_out, c.joined_at, i.last_in
		FROM contacts c
		LEFT JOIN (
			SELECT phone, MAX(sent_at) as last_msg
			FROM messages_log
			GROUP BY phone
		) m ON c.phone = m.phone
		-- Last ten digits, matching GetPhonesInServiceWindow, so the panel and
		-- the sender agree on who is inside the window.
		LEFT JOIN (
			SELECT RIGHT(phone, 10) AS phone10, MAX(sent_at) AS last_in
			FROM messages_log
			WHERE direction = 'incoming'
			GROUP BY 1
		) i ON RIGHT(c.phone, 10) = i.phone10
		ORDER BY COALESCE(m.last_msg, c.joined_at) DESC
	`
	rows, err := Pool.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		var c Contact
		err := rows.Scan(&c.ID, &c.Phone, &c.Name, &c.Company, &c.OptOut, &c.JoinedAt, &c.LastIncomingAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/contacts.go", err)
			continue
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}
