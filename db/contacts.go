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

func UpsertContact(phone, name, company string) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO contacts (phone, name, company) VALUES ($1, $2, $3) ON CONFLICT (phone) DO UPDATE SET name = EXCLUDED.name, company = EXCLUDED.company",
		phone, name, company)
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

func GetAllPhones() (map[string]bool, error) {
	rows, err := Pool.Query(context.Background(), "SELECT phone FROM contacts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phones := make(map[string]bool)
	for rows.Next() {
		var phone string
		if err := rows.Scan(&phone); err != nil {
			continue
		}
		phones[phone] = true
	}
	return phones, nil
}

func InsertContactIfAbsent(phone, name, company string) (bool, error) {
	tag, err := Pool.Exec(context.Background(),
		"INSERT INTO contacts (phone, name, company) VALUES ($1, $2, $3) ON CONFLICT (phone) DO NOTHING",
		phone, name, company)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func GetAllContacts() ([]Contact, error) {
	// Sync names before fetching
	SyncNamesFromLeads()

	query := `
		SELECT c.id, c.phone, c.name, c.company, c.opt_out, c.joined_at
		FROM contacts c
		LEFT JOIN (
			SELECT phone, MAX(sent_at) as last_msg
			FROM messages_log
			GROUP BY phone
		) m ON c.phone = m.phone
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
		err := rows.Scan(&c.ID, &c.Phone, &c.Name, &c.Company, &c.OptOut, &c.JoinedAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/contacts.go", err)
			continue
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}
