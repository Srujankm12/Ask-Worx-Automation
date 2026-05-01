package main

import (
	"askworx-whatsapp-bot/db"
	"context"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	db.InitDB()
	
	_, err := db.Pool.Exec(context.Background(), "ALTER TABLE contacts ADD COLUMN opt_out BOOLEAN DEFAULT FALSE;")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Added opt_out column")
}
