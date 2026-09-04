package db

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Campaign struct {
	ID            int       `json:"id"`
	Type          string    `json:"type"` // "quiz" or "poster"
	Question      string    `json:"question"`
	OptionA       string    `json:"option_a"`
	OptionB       string    `json:"option_b"`
	OptionC       string    `json:"option_c"`
	CorrectAnswer string    `json:"correct_answer"`
	Explanation   string    `json:"explanation"`
	YouTubeLink   string    `json:"youtube_link"`
	ImageURL      string    `json:"image_url"`
	Caption       string    `json:"caption"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Status        string    `json:"status"` // scheduled | sending | sent | cancelled
	TotalSent     int       `json:"total_sent"`
	CreatedAt     time.Time `json:"created_at"`
}

type QuizResponseDetail struct {
	Phone     string `json:"phone"`
	Name      string `json:"name"`
	Answer    string `json:"answer"`
	IsCorrect bool   `json:"is_correct"`
}

type CampaignAnalytics struct {
	CampaignID   int                  `json:"campaign_id"`
	TotalSent    int                  `json:"total_sent"`
	TotalAnswers int                  `json:"total_answers"`
	Correct      int                  `json:"correct"`
	Incorrect    int                  `json:"incorrect"`
	AnswerA      int                  `json:"answer_a"`
	AnswerB      int                  `json:"answer_b"`
	AnswerC      int                  `json:"answer_c"`
	Responses    []QuizResponseDetail `json:"responses"`
}

// CreateCampaign inserts a new campaign and returns its ID.
func CreateCampaign(c Campaign) (int, error) {
	query := `INSERT INTO campaigns
		(type, question, option_a, option_b, option_c, correct_answer, explanation, youtube_link, image_url, caption, scheduled_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`
	var id int
	err := Pool.QueryRow(context.Background(), query,
		c.Type, c.Question, c.OptionA, c.OptionB, c.OptionC,
		c.CorrectAnswer, c.Explanation, c.YouTubeLink,
		c.ImageURL, c.Caption, c.ScheduledAt,
	).Scan(&id)
	return id, err
}

// GetAllCampaigns returns campaigns ordered newest first.
func GetAllCampaigns() ([]Campaign, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, type, question, option_a, option_b, option_c, correct_answer, explanation,
		        youtube_link, image_url, caption, scheduled_at, status, total_sent, created_at
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
		        youtube_link, image_url, caption, scheduled_at, status, total_sent, created_at
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

// GetCampaignAnalytics returns quiz response stats for a campaign.
func GetCampaignAnalytics(campaignID int) (CampaignAnalytics, error) {
	a := CampaignAnalytics{CampaignID: campaignID}

	// Get total_sent from campaign row
	Pool.QueryRow(context.Background(),
		`SELECT total_sent FROM campaigns WHERE id = $1`, campaignID).Scan(&a.TotalSent)

	// Quiz specific — filter responses by checking which quiz rows belong to this campaign
	rows, err := Pool.Query(context.Background(),
		`SELECT qr.phone, 
		        COALESCE(NULLIF(c.name, ''), NULLIF(l.name, ''), 'Unknown'), 
		        qr.answer, qr.is_correct 
		 FROM quiz_responses qr
		 LEFT JOIN contacts c ON qr.phone = c.phone
		 LEFT JOIN (
		     SELECT DISTINCT ON (phone) phone, name 
		     FROM leads 
		     ORDER BY phone, created_at DESC
		 ) l ON qr.phone = l.phone
		 WHERE qr.quiz_id IN (SELECT id FROM quizzes WHERE campaign_id = $1)`, campaignID)
	if err != nil {
		return a, nil // analytics unavailable for posters
	}
	defer rows.Close()

	for rows.Next() {
		var detail QuizResponseDetail
		rows.Scan(&detail.Phone, &detail.Name, &detail.Answer, &detail.IsCorrect)

		a.Responses = append(a.Responses, detail)
		a.TotalAnswers++
		if detail.IsCorrect {
			a.Correct++
		} else {
			a.Incorrect++
		}
		switch detail.Answer {
		case "A":
			a.AnswerA++
		case "B":
			a.AnswerB++
		case "C":
			a.AnswerC++
		}
	}
	return a, nil
}

func GetCampaignsPaginated(limit, offset int, start, end string) ([]Campaign, error) {
	query := `SELECT id, type, question, option_a, option_b, option_c, correct_answer, explanation,
		        youtube_link, image_url, caption, scheduled_at, status, total_sent, created_at
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

// DeactivateAllQuizzes marks all quizzes as inactive before activating a new one.
func DeactivateAllQuizzes() {
	Pool.Exec(context.Background(), `UPDATE quizzes SET is_active = false`)
}

// CreateQuizFromCampaign inserts a new active quiz row from a campaign and returns its ID.
func CreateQuizFromCampaign(c Campaign) (int, error) {
	query := `INSERT INTO quizzes (campaign_id, question, option_a, option_b, option_c, correct_answer, explanation, youtube_link, is_active)
	          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true) RETURNING id`
	var id int
	err := Pool.QueryRow(context.Background(), query,
		c.ID, c.Question, c.OptionA, c.OptionB, c.OptionC,
		c.CorrectAnswer, c.Explanation, c.YouTubeLink,
	).Scan(&id)
	return id, err
}
