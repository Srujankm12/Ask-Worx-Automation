package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"askworx-whatsapp-bot/db"
)

// Sign-in for the console.
//
// What this replaces: the panel used to post a single shared password, the
// server compared it against ADMIN_PASSWORD, and on success handed back
// API_SECRET itself as the "token" — one value, never expiring, identical for
// everybody, and with a hardcoded fallback of "dummy-token-askworx" in the
// source whenever the environment was missing. An unset ADMIN_PASSWORD also
// meant an empty password matched.
//
// Now: accounts live in admin_users with bcrypt hashes, sign-in takes an email
// and a password, and what comes back is a session token that is signed,
// carries the address it was issued to, and expires.

const sessionTTL = 12 * time.Hour

// tokenVersion prefixes every token. Bumping it invalidates all outstanding
// sessions at once, which is what you want after a secret rotation.
const tokenVersion = "v1"

var errBadToken = errors.New("invalid session token")

// sessionSecret is read once at startup. main refuses to boot without it, so
// there is no fallback here to be forgotten about in production.
func sessionSecret() []byte {
	return []byte(os.Getenv("SESSION_SECRET"))
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, sessionSecret())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// issueToken returns "v1.<payload>.<signature>", where the payload carries the
// address and an absolute expiry. Nothing secret is inside it — the signature
// is what makes it unforgeable.
func issueToken(email string) string {
	payload := base64.RawURLEncoding.EncodeToString(
		fmt.Appendf(nil, "%s|%d", db.NormaliseEmail(email), time.Now().Add(sessionTTL).Unix()),
	)
	return tokenVersion + "." + payload + "." + sign(payload)
}

// verifyToken checks the signature before it trusts any field inside, and
// compares with hmac.Equal so the comparison is constant time.
func verifyToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenVersion {
		return "", errBadToken
	}

	expected := sign(parts[1])
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", errBadToken
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errBadToken
	}
	fields := strings.SplitN(string(raw), "|", 2)
	if len(fields) != 2 {
		return "", errBadToken
	}

	expiry, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return "", errBadToken
	}
	if time.Now().After(time.Unix(expiry, 0)) {
		return "", errBadToken
	}

	return fields[0], nil
}

// HashPassword is the one place a password becomes a hash.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// EnsureFirstAdmin seeds the very first account from the environment so a
// fresh deployment has a way in. It runs only when the table is empty — once
// somebody has signed in and changed their password, changing ADMIN_PASSWORD
// in the environment does nothing, which is the point.
func EnsureFirstAdmin() error {
	count, err := db.CountAdmins()
	if err != nil {
		return fmt.Errorf("could not read admin_users: %w", err)
	}
	if count > 0 {
		return nil
	}

	email := db.NormaliseEmail(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		return errors.New(
			"no admin accounts exist and ADMIN_EMAIL / ADMIN_PASSWORD are not both set — " +
				"set them once so the first account can be created, then sign in")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("could not hash the first admin password: %w", err)
	}
	if err := db.CreateAdmin(email, hash); err != nil {
		return fmt.Errorf("could not create the first admin: %w", err)
	}

	log.Printf("🔑 Created the first admin account for %s. Remove ADMIN_PASSWORD from the environment once you have signed in.", email)
	return nil
}

// AuthHandler signs somebody in.
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// The decode error used to be discarded, which meant a malformed body
	// arrived as an empty password — and an empty password matched an unset
	// ADMIN_PASSWORD.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "That sign-in request could not be read.")
		return
	}

	email := db.NormaliseEmail(body.Email)
	if email == "" || body.Password == "" {
		writeJSONError(w, http.StatusUnauthorized, "Enter both your email address and your password.")
		return
	}

	admin, err := db.GetAdminByEmail(email)
	if err != nil {
		if !errors.Is(err, db.ErrAdminNotFound) {
			log.Printf("[Auth] Lookup failed for a sign-in attempt: %v", err)
			writeJSONError(w, http.StatusInternalServerError,
				"We could not check those details just now. Please try again.")
			return
		}
		// Spend the same work on an unknown address as on a real one, so the
		// response time does not say whether the account exists.
		bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"),
			[]byte(body.Password))
		writeJSONError(w, http.StatusUnauthorized, "That email address and password do not match.")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(body.Password)) != nil {
		writeJSONError(w, http.StatusUnauthorized, "That email address and password do not match.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":      issueToken(admin.Email),
		"email":      admin.Email,
		"expires_in": int(sessionTTL.Seconds()),
	})
}

// AuthMiddleware guards every /api route except sign-in and the public
// uploads and campaign images that Meta has to be able to fetch.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/login" || path == "/login" ||
			strings.HasPrefix(path, "/uploads") ||
			strings.HasPrefix(path, "/api/uploads") ||
			(strings.HasPrefix(path, "/api/campaigns/") && strings.HasSuffix(path, "/image")) {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "You are not signed in.")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSONError(w, http.StatusUnauthorized, "You are not signed in.")
			return
		}

		if _, err := verifyToken(strings.TrimSpace(parts[1])); err != nil {
			writeJSONError(w, http.StatusUnauthorized, "Your session has expired. Please sign in again.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// constantTimeEqual is used where a non-session secret is compared.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
