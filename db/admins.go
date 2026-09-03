package db

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// AdminUser is somebody who can sign in to the console.
//
// Passwords are never stored, only their bcrypt hash, and PasswordHash is
// deliberately not tagged for JSON so it cannot leak through a handler that
// marshals this struct by accident.
type AdminUser struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// ErrAdminNotFound separates "no such account" from a database failure, so a
// caller can answer a sign-in attempt without leaking which one it was.
var ErrAdminNotFound = errors.New("admin user not found")

// NormaliseEmail is the single definition of how an address is compared.
// Addresses are stored and looked up lowercased and trimmed, so signing in as
// "Admin@Askworx.in " reaches the same account as "admin@askworx.in".
func NormaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GetAdminByEmail returns the account for an address, or ErrAdminNotFound.
func GetAdminByEmail(email string) (AdminUser, error) {
	var a AdminUser
	err := Pool.QueryRow(context.Background(),
		`SELECT id, email, password_hash, created_at FROM admin_users WHERE email = $1`,
		NormaliseEmail(email),
	).Scan(&a.ID, &a.Email, &a.PasswordHash, &a.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrAdminNotFound
	}
	return a, err
}

// CreateAdmin adds an account. The caller hashes the password.
func CreateAdmin(email, passwordHash string) error {
	_, err := Pool.Exec(context.Background(),
		`INSERT INTO admin_users (email, password_hash) VALUES ($1, $2)
		 ON CONFLICT (email) DO NOTHING`,
		NormaliseEmail(email), passwordHash)
	return err
}

// UpdateAdminPassword replaces the stored hash for an address.
func UpdateAdminPassword(email, passwordHash string) error {
	_, err := Pool.Exec(context.Background(),
		`UPDATE admin_users SET password_hash = $1 WHERE email = $2`,
		passwordHash, NormaliseEmail(email))
	return err
}

// CountAdmins reports how many accounts exist, so startup can tell a fresh
// deployment from one that has already been seeded.
func CountAdmins() (int, error) {
	var n int
	err := Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n, err
}

// ListAdmins returns every account, newest last. Hashes are not included.
func ListAdmins() ([]AdminUser, error) {
	rows, err := Pool.Query(context.Background(),
		`SELECT id, email, created_at FROM admin_users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	admins := []AdminUser{}
	for rows.Next() {
		var a AdminUser
		if err := rows.Scan(&a.ID, &a.Email, &a.CreatedAt); err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}
