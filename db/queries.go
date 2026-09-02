package db

import (
	"context"
	"log"
)

type CustomerQuery struct {
	ID              int
	PhoneNumber     string
	UserName        string
	Category        string
	OriginalMessage string
	Status          string
}

func SaveCustomerQuery(query CustomerQuery) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO customer_queries (phone_number, user_name, original_message, status) VALUES ($1, $2, $3, 'pending')",
		query.PhoneNumber, query.UserName, query.OriginalMessage)
	if err != nil {
		log.Printf("[DB] Error saving query: %v", err)
	}
	return err
}

func GetLatestPendingQuery(phone string) (CustomerQuery, error) {
	var q CustomerQuery
	err := Pool.QueryRow(context.Background(),
		"SELECT id, phone_number, original_message FROM customer_queries WHERE phone_number = $1 AND category IS NULL ORDER BY created_at DESC LIMIT 1",
		phone).Scan(&q.ID, &q.PhoneNumber, &q.OriginalMessage)
	return q, err
}

func UpdateQueryCategory(id int, category string) error {
	_, err := Pool.Exec(context.Background(),
		"UPDATE customer_queries SET category = $1 WHERE id = $2",
		category, id)
	return err
}
