package main

import (
	"askworx-whatsapp-bot/db"
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	db.InitDB()
	
	// Query all settings directly
	rows, err := db.Pool.Query(context.Background(), "SELECT key, value FROM settings")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- All Settings ---")
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %s\n", key, val)
	}
}
