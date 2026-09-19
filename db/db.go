package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func InitDB() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return err
	}

	Pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return err
	}

	// pgxpool connects lazily, so without this the process starts and reports
	// healthy against a database it can never reach, and every request fails
	// one at a time instead.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Pool.Ping(ctx); err != nil {
		return fmt.Errorf("could not reach the database: %w", err)
	}

	// Run migration
	// A failed migration used to print "Migration warning" and carry on, which
	// left the service running against a schema that did not match the code.
	migration, err := os.ReadFile("migration.sql")
	if err != nil {
		return fmt.Errorf("could not read migration.sql (it must ship alongside the binary): %w", err)
	}
	if _, err := Pool.Exec(context.Background(), string(migration)); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Hot-fix: Ensure campaign_id exists in quizzes
	Pool.Exec(context.Background(), `ALTER TABLE quizzes ADD COLUMN IF NOT EXISTS campaign_id INTEGER REFERENCES campaigns(id)`)

	// Hot-fix: posters now carry a title and description instead of a single
	// caption. caption is kept so campaigns scheduled before this change still
	// read and send correctly. DEFAULT '' (not NULL) so existing rows scan
	// cleanly into Go's plain string fields, the same way caption always has.
	Pool.Exec(context.Background(), `ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS title TEXT DEFAULT ''`)
	Pool.Exec(context.Background(), `ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''`)

	// Hot-fix: Create customer_queries table
	Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS customer_queries (
			id SERIAL PRIMARY KEY,
			phone_number TEXT NOT NULL,
			user_name TEXT,
			category TEXT,
			original_message TEXT,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// Create faqs table
	Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS faqs (
			id SERIAL PRIMARY KEY,
			keywords TEXT NOT NULL,
			answer TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)

	return Pool.Ping(context.Background())
}
