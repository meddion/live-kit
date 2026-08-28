package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionTTL = 12 * time.Hour

type session struct {
	username  string
	expiresAt time.Time
}

// SessionStore keeps authenticated sessions in memory keyed by an opaque token.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]session
	ttl      time.Duration
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]session), ttl: sessionTTL}
}

// Create issues a new session token for the user and returns it with its expiry.
func (this *SessionStore) Create(username string) (string, time.Time, error) {
	token, err := newSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt := time.Now().Add(this.ttl)
	this.mu.Lock()
	this.sessions[token] = session{username: username, expiresAt: expiresAt}
	this.mu.Unlock()

	return token, expiresAt, nil
}

// Validate returns the session's username if the token is known and unexpired.
func (this *SessionStore) Validate(token string) (string, bool) {
	this.mu.Lock()
	defer this.mu.Unlock()

	sess, ok := this.sessions[token]
	if !ok {
		return "", false
	}
	if time.Now().After(sess.expiresAt) {
		delete(this.sessions, token)
		return "", false
	}

	return sess.username, true
}

// Delete removes a session, if it exists.
func (this *SessionStore) Delete(token string) {
	this.mu.Lock()
	delete(this.sessions, token)
	this.mu.Unlock()
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
