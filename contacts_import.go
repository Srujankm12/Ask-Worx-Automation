package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"askworx-whatsapp-bot/db"
)

// Contact Import.
//
// The admin panel's Import Contacts wizard parses the uploaded file and lets
// the operator preview and hand-correct every row client-side, but that
// preview is only ever a preview: nothing is saved until this endpoint says
// so. It re-runs the same validation and duplicate checks the panel already
// ran — because a browser's classification of a row is not something the
// server can trust — and it is what actually decides, and records, what got
// imported.
//
// One request in, one row-by-row verdict out. There is no partial or
// resumable import: every row in the request is judged independently and the
// full result set is returned in the same response.

var nonDigits = regexp.MustCompile(`\D+`)

// normalizeImportPhone mirrors the admin panel's own normalizePhone(): digits
// only, so "+91 98765-43210" and "9198765 43210" compare equal to whatever is
// already stored.
func normalizeImportPhone(raw string) string {
	return nonDigits.ReplaceAllString(raw, "")
}

// isValidImportPhone mirrors the panel's isValidPhone() — the same 8–15
// digit range, so a row that looked valid in the preview does not turn
// invalid the moment it reaches the server.
func isValidImportPhone(digits string) bool {
	return len(digits) >= 8 && len(digits) <= 15
}

type importContactRow struct {
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
}

type importRowResult struct {
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
	Status  string `json:"status"` // imported | invalid | duplicate | failed
	Reason  string `json:"reason"`
}

type importResponse struct {
	Total    int               `json:"total"`
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"` // invalid + duplicate
	Failed   int               `json:"failed"`
	Results  []importRowResult `json:"results"`
}

// ImportContactsHandler validates and saves a batch of contacts in one
// request, returning a verdict for every row — imported, invalid (with the
// reason), a duplicate (of the database or of an earlier row in the same
// batch), or failed (a real database error on an otherwise-valid row).
func ImportContactsHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Contacts []importContactRow `json:"contacts"`
	}

	// A generous cap: even a few thousand rows of {row,name,phone,company}
	// JSON stays well under this, while still bounding the request.
	const maxImportBody = 10 << 20 // 10 MiB
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxImportBody)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "That import request could not be read.")
		return
	}
	if len(body.Contacts) == 0 {
		writeJSONError(w, http.StatusBadRequest, "No rows were sent to import.")
		return
	}

	existingPhones, err := db.GetAllPhones()
	if err != nil {
		log.Printf("[Import] Could not load existing contacts: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Could not check existing contacts. Please try again.")
		return
	}

	// existingPhones is a frozen snapshot of the database as of the start of
	// this batch — it is never written to. fileSeen tracks only phones this
	// batch itself has already imported, which is what lets a repeat inside
	// the file be told apart from one that was already on file: a phone
	// present in existingPhones but not fileSeen was a duplicate before this
	// import even started; one in fileSeen was made valid by an earlier row
	// in this same batch.
	fileSeen := make(map[string]bool)
	results := make([]importRowResult, 0, len(body.Contacts))
	var imported, invalid, duplicate, failed int

	for _, row := range body.Contacts {
		name := strings.TrimSpace(row.Name)
		company := strings.TrimSpace(row.Company)
		rawPhone := strings.TrimSpace(row.Phone)
		digits := normalizeImportPhone(rawPhone)

		result := importRowResult{Row: row.Row, Name: name, Phone: rawPhone, Company: company}

		switch {
		case name == "" && rawPhone == "" && company == "":
			result.Status, result.Reason = "invalid", "Empty row"
			invalid++

		case rawPhone == "":
			result.Status, result.Reason = "invalid", "Phone number is required"
			invalid++

		case !isValidImportPhone(digits):
			result.Status, result.Reason = "invalid", "Phone number looks invalid"
			invalid++

		case existingPhones[digits] || fileSeen[digits]:
			result.Status = "duplicate"
			if fileSeen[digits] {
				result.Reason = "Duplicate in this file"
			} else {
				result.Reason = "Already exists in Contacts"
			}
			duplicate++

		default:
			inserted, err := db.InsertContactIfAbsent(digits, name, company)
			switch {
			case err != nil:
				log.Printf("[Import] row %d: could not save contact: %v", row.Row, err)
				result.Status, result.Reason = "failed", "Could not be saved — please try again."
				failed++
			case !inserted:
				// Lost a race against a concurrent insert of the same number.
				result.Status, result.Reason = "duplicate", "Already exists in Contacts"
				duplicate++
			default:
				result.Status, result.Reason = "imported", ""
				imported++
				fileSeen[digits] = true
			}
		}

		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(importResponse{
		Total:    len(body.Contacts),
		Imported: imported,
		Skipped:  invalid + duplicate,
		Failed:   failed,
		Results:  results,
	})
}
