package db

import (
	"context"
	"fmt"
	"log"
	"time"
)

func CreateLead(phone, name, company, requirement, contactPhone string) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO leads (phone, name, company, requirement, contact_phone) VALUES ($1, $2, $3, $4, $5)",
		phone, name, company, requirement, contactPhone)
	if err != nil {
		fmt.Printf("ERROR creating lead: %v\n", err)
	}
	return err
}

type Lead struct {
	ID           int       `json:"id"`
	Phone        string    `json:"phone"`
	Name         *string   `json:"name"`
	Company      *string   `json:"company"`
	Requirement  *string   `json:"requirement"`
	ContactPhone *string   `json:"contact_phone"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func GetAllLeads() ([]Lead, error) {
	rows, err := Pool.Query(context.Background(), "SELECT id, phone, name, company, requirement, contact_phone, status, created_at FROM leads ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leads := []Lead{}
	for rows.Next() {
		var l Lead
		err := rows.Scan(&l.ID, &l.Phone, &l.Name, &l.Company, &l.Requirement, &l.ContactPhone, &l.Status, &l.CreatedAt)
		if err != nil {
			fmt.Printf("Error scanning lead row: %v\n", err)
			continue
		}
		leads = append(leads, l)
	}
	return leads, nil
}

func UpdateLeadStatus(id int, status string) error {
	_, err := Pool.Exec(context.Background(), "UPDATE leads SET status = $1 WHERE id = $2", status, id)
	return err
}

func GetLeadsPaginated(limit, offset int, start, end string) ([]Lead, error) {
	query := `SELECT id, phone, name, company, requirement, contact_phone, status, created_at FROM leads WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND created_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND created_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leads := []Lead{}
	for rows.Next() {
		var l Lead
		err := rows.Scan(&l.ID, &l.Phone, &l.Name, &l.Company, &l.Requirement, &l.ContactPhone, &l.Status, &l.CreatedAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/leads.go", err)
			continue
		}
		leads = append(leads, l)
	}
	return leads, nil
}

func GetTotalLeadsCount(start, end string) (int, error) {
	query := `SELECT COUNT(*) FROM leads WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND created_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND created_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	var count int
	err := Pool.QueryRow(context.Background(), query, args...).Scan(&count)
	return count, err
}

func GetNewLeadsCount() (int, error) {
	var count int
	err := Pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM leads WHERE status = 'new'").Scan(&count)
	return count, err
}
