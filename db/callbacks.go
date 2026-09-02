package db

import (
	"context"
	"log"
	"time"
)

func CreateCallback(phone, name, preferredTime string) error {
	_, err := Pool.Exec(context.Background(),
		"INSERT INTO callbacks (phone, name, preferred_time) VALUES ($1, $2, $3)",
		phone, name, preferredTime)
	return err
}

type Callback struct {
	ID            int       `json:"id"`
	Phone         string    `json:"phone"`
	Name          string    `json:"name"`
	PreferredTime string    `json:"preferred_time"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func GetAllCallbacks() ([]Callback, error) {
	rows, err := Pool.Query(context.Background(), "SELECT id, phone, name, preferred_time, status, created_at FROM callbacks ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	callbacks := []Callback{}
	for rows.Next() {
		var c Callback
		err := rows.Scan(&c.ID, &c.Phone, &c.Name, &c.PreferredTime, &c.Status, &c.CreatedAt)
		if err != nil {
			log.Printf("[db] %s: skipping unreadable row: %v", "db/callbacks.go", err)
			continue
		}
		callbacks = append(callbacks, c)
	}
	return callbacks, nil
}

func MarkCallbackDone(id int) error {
	_, err := Pool.Exec(context.Background(), "UPDATE callbacks SET status = 'done' WHERE id = $1", id)
	return err
}
