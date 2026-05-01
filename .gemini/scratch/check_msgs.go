package main

import (
	"askworx-whatsapp-bot/db"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	db.InitDB()
	
	// Query recent messages
	rows, err := db.Pool.Query(context.Background(), "SELECT phone, direction, message, sent_at FROM messages_log ORDER BY sent_at DESC LIMIT 20")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- Recent Messages ---")
	for rows.Next() {
		var phone, direction, content string
		var created_at time.Time
		if err := rows.Scan(&phone, &direction, &content, &created_at); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("[%s] %s %s: %s\n", created_at.Format(time.RFC3339), direction, phone, content)
	}
}
