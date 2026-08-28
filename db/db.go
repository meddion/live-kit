// Package db provides SQLite-backed storage for users and permissions using
// the pure-Go modernc.org/sqlite driver.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meddion/live-kit/api"
	"github.com/meddion/live-kit/auth"
	_ "modernc.org/sqlite"
)

// defaultPermissions is the set of permissions guaranteed to exist.
var defaultPermissions = []api.Permission{
	api.PermViewRooms,
	api.PermJoinRooms,
	api.PermCreateRooms,
	api.PermGodAlmighty,
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS permissions (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS user_permissions (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);
`

// Store wraps a SQLite database holding users and their permissions.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and applies the
// schema plus the default permission rows.
func Open(c context.Context, path string) (*Store, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	if _, err := sqlDB.ExecContext(c, "PRAGMA foreign_keys = ON;"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	s := &Store{db: sqlDB}
	if err := s.migrate(c); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (this *Store) Close() error {
	return this.db.Close()
}

func (this *Store) migrate(c context.Context) error {
	if _, err := this.db.Exec(schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	for _, name := range defaultPermissions {
		if _, err := this.db.ExecContext(
			c,
			"INSERT OR IGNORE INTO permissions (name) VALUES (?)", name,
		); err != nil {
			return fmt.Errorf("seeding permission %q: %w", name, err)
		}
	}
	return nil
}

// Authenticate reports whether the username/password pair matches an active
// user. Passwords are verified against the stored bcrypt hash.
func (this *Store) Authenticate(c context.Context, username, password string) bool {
	var (
		hash   string
		active bool
	)
	err := this.db.QueryRowContext(
		c,
		"SELECT password_hash, active FROM users WHERE name = ?", username,
	).Scan(&hash, &active)
	if err != nil || !active {
		return false
	}
	return auth.CompareHashAndPassword(hash, password) == nil
}

// HasPermission reports whether the user holds the named permission. A user
// with GodAlmighty is treated as holding every permission.
func (this *Store) HasPermission(c context.Context, username string, perm api.Permission) (bool, error) {
	const query = `
SELECT EXISTS (
    SELECT 1
    FROM users u
    JOIN user_permissions up ON up.user_id = u.id
    JOIN permissions p       ON p.id = up.permission_id
    WHERE u.name = ?
      AND u.active = 1
      AND p.name IN (?, ?)
)`
	var ok bool
	if err := this.db.QueryRowContext(c, query, username, perm, api.PermGodAlmighty).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

// Permissions returns the permission names granted to the user.
func (this *Store) Permissions(c context.Context, username string) ([]api.Permission, error) {
	const query = `
SELECT p.name
FROM users u
JOIN user_permissions up ON up.user_id = u.id
JOIN permissions p       ON p.id = up.permission_id
WHERE u.name = ? AND u.active = 1
ORDER BY p.name`
	rows, err := this.db.QueryContext(c, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []api.Permission
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		perms = append(perms, api.Permission(name))
	}
	return perms, rows.Err()
}
