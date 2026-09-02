package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"askworx-whatsapp-bot/db"
)

type Button struct {
	ID    string
	Title string
}

func sendTextMessage(to, message string) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"body": message,
		},
	}
	sendToMeta(payload, to, message)
}

func sendImage(to, imageURL, caption string) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "image",
		"image": map[string]string{
			"link":    imageURL,
			"caption": caption,
		},
	}
	sendToMeta(payload, to, "[Image: "+imageURL+"] "+caption)
}

func sendInteractiveButtons(to, bodyText string, buttons []Button) {
	var waButtons []map[string]interface{}
	for _, b := range buttons {
		// Ensure title is max 20 chars
		title := b.Title
		if len([]rune(title)) > 20 {
			title = string([]rune(title)[:17]) + "..."
		}

		waButtons = append(waButtons, map[string]interface{}{
			"type": "reply",
			"reply": map[string]string{
				"id":    b.ID,
				"title": title,
			},
		})
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "interactive",
		"interactive": map[string]interface{}{
			"type": "button",
			"body": map[string]string{
				"text": bodyText,
			},
			"action": map[string]interface{}{
				"buttons": waButtons,
			},
		},
	}
	sendToMeta(payload, to, bodyText)
}

func sendImageWithButtons(to, imageURL, bodyText string, buttons []Button) {
	var waButtons []map[string]interface{}
	for _, b := range buttons {
		title := b.Title
		if len([]rune(title)) > 20 {
			title = string([]rune(title)[:17]) + "..."
		}

		waButtons = append(waButtons, map[string]interface{}{
			"type": "reply",
			"reply": map[string]string{
				"id":    b.ID,
				"title": title,
			},
		})
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "interactive",
		"interactive": map[string]interface{}{
			"type": "button",
			"header": map[string]interface{}{
				"type":  "image",
				"image": map[string]string{"link": imageURL},
			},
			"body": map[string]string{
				"text": bodyText,
			},
			"action": map[string]interface{}{
				"buttons": waButtons,
			},
		},
	}
	sendToMeta(payload, to, "[Image: "+imageURL+"] "+bodyText)
}

func sendToMeta(payload map[string]interface{}, to, logMsg string) {
	url := fmt.Sprintf("https://graph.facebook.com/v17.0/%s/messages", os.Getenv("PHONE_NUMBER_ID"))
	token := os.Getenv("ACCESS_TOKEN")

	// Normalize phone number for Meta (must include country code, no +, no spaces)
	normalizedTo := strings.ReplaceAll(to, "+", "")
	normalizedTo = strings.ReplaceAll(normalizedTo, " ", "")
	if len(normalizedTo) == 10 {
		normalizedTo = "91" + normalizedTo
	}
	payload["to"] = normalizedTo

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending to Meta:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Meta API Error (Status %d): %s\n", resp.StatusCode, string(bodyBytes))
		log.Printf("❌ Failed Payload: %s\n", string(jsonPayload))
		return
	}

	db.LogMessage(to, "outgoing", logMsg)
	log.Printf("✅ Meta Success! Message sent to %s: %s\n", to, logMsg)
}

func createCleanID(text string) string {
	var parts []string
	curr := ""
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			curr += string(r)
		} else if len(curr) > 0 {
			parts = append(parts, curr)
			curr = ""
		}
	}
	if curr != "" {
		parts = append(parts, curr)
	}
	res := strings.Join(parts, "_")
	if res == "" {
		return "btn"
	}
	return res
}
