package db

import (
	"context"
	"log"
	"time"
)

// Template is a broadcast saved to send again.
//
// It holds the words and the address of an image, never the image bytes. A
// poster uploaded from the panel is already served at /api/campaigns/{id}/image
// and that URL is what gets saved here, so saving a template copies nothing.
type Template struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	Links       string    `json:"links"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetTemplates returns saved templates, newest first.
func GetTemplates() ([]Template, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, name, title, description, image_url, links, created_at
		 FROM broadcast_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Title, &t.Description, &t.ImageURL, &t.Links, &t.CreatedAt); err != nil {
			log.Printf("[Templates] skipping unreadable row: %v", err)
			continue
		}
		templates = append(templates, t)
	}
	return templates, nil
}

// CreateTemplate saves a template and returns its ID.
func CreateTemplate(t Template) (int, error) {
	var id int
	err := Pool.QueryRow(context.Background(),
		`INSERT INTO broadcast_templates (name, title, description, image_url, links)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		t.Name, t.Title, t.Description, t.ImageURL, t.Links).Scan(&id)
	return id, err
}

// DeleteTemplate removes a saved template.
func DeleteTemplate(id int) error {
	_, err := Pool.Exec(context.Background(),
		`DELETE FROM broadcast_templates WHERE id = $1`, id)
	return err
}
