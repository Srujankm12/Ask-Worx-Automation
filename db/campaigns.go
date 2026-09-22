package db

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Campaign struct {
	ID            int    `json:"id"`
	Type          string `json:"type"` // always "poster"
	Question      string `json:"question"`
	OptionA       string `json:"option_a"`
	OptionB       string `json:"option_b"`
	OptionC       string `json:"option_c"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
	YouTubeLink   string `json:"youtube_link"`
	ImageURL      string `json:"image_url"`
	Caption       string `json:"caption"` // legacy single-field posters, pre-title/description
	Title         string `json:"title"`
	Description   string `json:"description"`
	// The closing lines under the text. Empty means the poster falls back to
	// the website and email held in settings.
	Links string `json:"links"`
	// Metadata for a poster image stored directly in the database (image_data,
	// scanned separately by GetCampaignImage — never through this struct, so a
	// campaigns list response never carries a multi-hundred-KB base64 blob).
	ImageName   string    `json:"image_name,omitempty"`
	ImageType   string    `json:"image_type,omitempty"`
	ImageSize   int64     `json:"image_size,omitempty"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"` // scheduled | sending | sent | cancelled
	TotalSent   int       `json:"total_sent"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateCampaign inserts a new campaign and returns its ID. imageData is the
// raw bytes of an uploaded poster image, nil when the poster uses image_url
// (a pasted link, or one already rewritten by the caller) instead.
func CreateCampaign(c Campaign, imageData []byte) (int, error) {
	query := `INSERT INTO campaigns
		(type, question, option_a, option_b, option_c, correct_answer, explanation, youtube_link, image_url, caption, title, description, links, scheduled_at, image_name, image_type, image_size, image_data)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id`
	var id int
	err := Pool.QueryRow(context.Background(), query,
		c.Type, c.Question, c.OptionA, c.OptionB, c.OptionC,
		c.CorrectAnswer, c.Explanation, c.YouTubeLink,
		c.ImageURL, c.Caption, c.Title, c.Description, c.Links, c.ScheduledAt,
		c.ImageName, c.ImageType, c.ImageSize, imageData,
	).Scan(&id)
	return id, err
}

// SetCampaignImageURL points a campaign at the endpoint that serves its own
// stored image, once the row (and its id) exists.
func SetCampaignImageURL(id int, url string) error {
	_, err := Pool.Exec(context.Background(),
		`UPDATE campaigns SET image_url = $1 WHERE id = $2`, url, id)
	return err
}

// GetCampaignImage reads back the binary image stored for a campaign. It is
// its own query, never folded into the list/paginated selects, so listing
// campaigns never pulls image bytes over the wire.
func GetCampaignImage(id int) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := Pool.QueryRow(context.Background(),
		`SELECT image_data, image_type FROM campaigns WHERE id = $1`, id).Scan(&data, &contentType)
	return data, contentType, err
}

// GetAllCampaigns returns campaigns ordered newest first.
func GetAllCampaigns() ([]Campaign, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, type, question, option_a, option_b, option_c, correct_answer, explanation,
		        youtube_link, image_url, caption, title, description, links, image_name, image_type, image_size,
		        scheduled_at, status, total_sent, created_at
		 FROM campaigns ORDER BY scheduled_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		err := rows.Scan(&c.ID, &c.Type, &c.Question, &c.OptionA, &c.OptionB, &c.OptionC,
			&c.CorrectAnswer, &c.Explanation, &c.YouTubeLink, &c.ImageURL, &c.Caption,
			&c.Title, &c.Description, &c.Links, &c.ImageName, &c.ImageType, &c.ImageSize,
			&c.ScheduledAt, &c.Status, &c.TotalSent, &c.CreatedAt)
		if err != nil {
			// Silently dropping the row hid a whole page of campaigns behind a
			// 200 with an empty list. Skipping is still the right recovery, but
			// it has to leave a trace.
			log.Printf("[Campaigns] skipping unreadable row: %v", err)
			continue
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// GetDueCampaigns returns scheduled campaigns whose time has arrived.
func GetDueCampaigns() ([]Campaign, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, type, question, option_a, option_b, option_c, correct_answer, explanation,
		        youtube_link, image_url, caption, title, description, links, image_name, image_type, image_size,
		        scheduled_at, status, total_sent, created_at
		 FROM campaigns WHERE status = 'scheduled' AND scheduled_at <= NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		rows.Scan(&c.ID, &c.Type, &c.Question, &c.OptionA, &c.OptionB, &c.OptionC,
			&c.CorrectAnswer, &c.Explanation, &c.YouTubeLink, &c.ImageURL, &c.Caption,
			&c.Title, &c.Description, &c.Links, &c.ImageName, &c.ImageType, &c.ImageSize,
			&c.ScheduledAt, &c.Status, &c.TotalSent, &c.CreatedAt)
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// ClaimCampaign takes a due campaign out of the queue before anything is sent,
// and reports whether this caller is the one that got it.
//
// The broadcaster cron runs every minute, and MarkCampaignSent was only called
// after the whole send loop finished. A broadcast to a few hundred contacts is
// a few hundred serial calls to Meta and takes well over a minute, so the next
// tick re-selected the same 'scheduled' row and sent the entire campaign a
// second time. The UPDATE is guarded on the current status and the database
// applies it atomically, so exactly one caller sees a row affected — which
// also makes this safe if the service is ever run as more than one instance.
//
// A campaign left in 'sending' means the process died mid-broadcast. That is
// deliberately not retried automatically: re-sending to contacts who already
// received it is worse than leaving it visible in the panel for someone to
// decide about.
func ClaimCampaign(id int) (bool, error) {
	tag, err := Pool.Exec(context.Background(),
		`UPDATE campaigns SET status = 'sending' WHERE id = $1 AND status = 'scheduled'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// MarkCampaignSent updates status to 'sent' and records total_sent count.
func MarkCampaignSent(id, totalSent int) error {
	_, err := Pool.Exec(context.Background(),
		`UPDATE campaigns SET status = 'sent', total_sent = $1 WHERE id = $2`, totalSent, id)
	return err
}

// CancelCampaign marks a scheduled campaign as cancelled.
func CancelCampaign(id int) error {
	_, err := Pool.Exec(context.Background(),
		`UPDATE campaigns SET status = 'cancelled' WHERE id = $1 AND status = 'scheduled'`, id)
	return err
}

func GetCampaignsPaginated(limit, offset int, start, end string) ([]Campaign, error) {
	query := `SELECT id, type, question, option_a, option_b, option_c, correct_answer, explanation,
		        youtube_link, image_url, caption, title, description, links, image_name, image_type, image_size,
		        scheduled_at, status, total_sent, created_at
		 FROM campaigns WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND scheduled_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND scheduled_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	query += fmt.Sprintf(" ORDER BY scheduled_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		var c Campaign
		err := rows.Scan(&c.ID, &c.Type, &c.Question, &c.OptionA, &c.OptionB, &c.OptionC,
			&c.CorrectAnswer, &c.Explanation, &c.YouTubeLink, &c.ImageURL, &c.Caption,
			&c.Title, &c.Description, &c.Links, &c.ImageName, &c.ImageType, &c.ImageSize,
			&c.ScheduledAt, &c.Status, &c.TotalSent, &c.CreatedAt)
		if err != nil {
			// Silently dropping the row hid a whole page of campaigns behind a
			// 200 with an empty list. Skipping is still the right recovery, but
			// it has to leave a trace.
			log.Printf("[Campaigns] skipping unreadable row: %v", err)
			continue
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

func GetTotalCampaignsCount(start, end string) (int, error) {
	query := `SELECT COUNT(*) FROM campaigns WHERE 1=1`
	args := []interface{}{}
	argID := 1

	if start != "" {
		query += fmt.Sprintf(" AND scheduled_at >= $%d", argID)
		args = append(args, start)
		argID++
	}
	if end != "" {
		query += fmt.Sprintf(" AND scheduled_at <= $%d", argID)
		args = append(args, end)
		argID++
	}

	var count int
	err := Pool.QueryRow(context.Background(), query, args...).Scan(&count)
	return count, err
}
