package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/meddion/live-kit/auth"
)

type seedFile struct {
	Users []struct {
		Username    string   `json:"username"`
		Password    string   `json:"password"`
		Permissions []string `json:"permissions"`
	} `json:"users"`
}

// Seed loads users from the JSON file at path, hashing their plaintext
// passwords and granting the listed permissions. Existing users are left
// untouched so seeding is idempotent.
func (this *Store) Seed(path string) error {
	buf, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Warn("seed file not found, skipping user seeding", "path", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading seed file: %w", err)
	}

	var parsed seedFile
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return fmt.Errorf("parsing seed file: %w", err)
	}
	if len(parsed.Users) == 0 {
		return errors.New("no users found in seed file")
	}

	tx, err := this.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, u := range parsed.Users {
		if u.Username == "" || u.Password == "" {
			continue
		}
		if err := seedUser(tx, u.Username, u.Password, u.Permissions); err != nil {
			return fmt.Errorf("seeding user %q: %w", u.Username, err)
		}
	}

	return tx.Commit()
}

func seedUser(tx *sql.Tx, username, password string, permissions []string) error {
	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO users (name, password_hash) VALUES (?, ?)",
		username, string(hash),
	); err != nil {
		return err
	}

	var userID int64
	if err := tx.QueryRow("SELECT id FROM users WHERE name = ?", username).Scan(&userID); err != nil {
		return err
	}

	for _, perm := range permissions {
		var permID int64
		err := tx.QueryRow("SELECT id FROM permissions WHERE name = ?", perm).Scan(&permID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("unknown permission %q", perm)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT OR IGNORE INTO user_permissions (user_id, permission_id) VALUES (?, ?)",
			userID, permID,
		); err != nil {
			return err
		}
	}
	return nil
}
