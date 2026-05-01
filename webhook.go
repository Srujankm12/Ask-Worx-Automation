package main

import (
	"askworx-whatsapp-bot/db"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type WebhookRequest struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					ID   string `json:"id"`
					From string `json:"from"`
					Type string `json:"type"`
					Text *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Interactive *struct {
						ButtonReply struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"button_reply"`
					} `json:"interactive,omitempty"`
					Location *struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
					} `json:"location,omitempty"`
					Image *struct {
						ID      string `json:"id"`
						Caption string `json:"caption"`
					} `json:"image,omitempty"`
					Document *struct {
						ID       string `json:"id"`
						Filename string `json:"filename"`
					} `json:"document,omitempty"`
				} `json:"messages"`
				Statuses []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Errors []struct {
						Code    int    `json:"code"`
						Title   string `json:"title"`
						Message string `json:"message"`
					} `json:"errors,omitempty"`
				} `json:"statuses,omitempty"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		verifyToken := os.Getenv("VERIFY_TOKEN")
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && token == verifyToken {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(challenge))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPost {
		var req WebhookRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusOK) // Always return 200
			return
		}

		for _, entry := range req.Entry {
			for _, change := range entry.Changes {
				// 1. Save contact names from profile
				for _, contact := range change.Value.Contacts {
					db.SaveContact(contact.WaID, contact.Profile.Name)
				}

				// 2. Process messages
				for _, msg := range change.Value.Messages {
					phone := msg.From
					text := ""
					var lat, lng float64

					if msg.Type == "location" && msg.Location != nil {
						lat = msg.Location.Latitude
						lng = msg.Location.Longitude
						text = "LOCATION_DATA"
					} else if msg.Type == "image" && msg.Image != nil {
						text = "[IMAGE_RECEIVED]"
						if msg.Image.Caption != "" {
							text += ": " + msg.Image.Caption
						}
					} else if msg.Type == "document" && msg.Document != nil {
						text = "[DOCUMENT_RECEIVED]"
						if msg.Document.Filename != "" {
							text += ": " + msg.Document.Filename
						}
					} else if msg.Text != nil {
						text = msg.Text.Body
					} else if msg.Interactive != nil {
						text = msg.Interactive.ButtonReply.ID
					} else {
						text = "[" + strings.ToUpper(msg.Type) + "_RECEIVED]"
					}

					if text != "" {
						log.Printf("[Webhook] Incoming from %s: %s", phone, text)
						db.LogMessage(phone, "incoming", text, msg.ID)
						go handleMessage(phone, text, lat, lng)
					}
				}
				
				// 3. Process delivery statuses
				for _, status := range change.Value.Statuses {
					if status.Status == "failed" && len(status.Errors) > 0 {
						errStr := ""
						for _, e := range status.Errors {
							errStr += e.Message + " "
						}
						log.Printf("⚠️ [Webhook] Message Delivery FAILED (ID: %s) - Reason: %s", status.ID, errStr)
					} else {
						// log.Printf("[Webhook] Message %s status: %s", status.ID, status.Status)
					}
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}
}
