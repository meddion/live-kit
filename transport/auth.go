package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/meddion/live-kit/auth"
)

type Authenticator interface {
	Authenticate(c context.Context, username, passwordHash string) bool
}

type SessionManager interface {
	Create(username string) (string, time.Time, error)
	Validate(token string) (string, bool)
	Delete(token string)
}

const sessionCookieName = "session"

type AuthMiddleware struct {
	users    Authenticator
	sessions SessionManager
	realm    string
}

func NewAuthMiddleware(users Authenticator, sessions SessionManager) *AuthMiddleware {
	return &AuthMiddleware{users: users, sessions: sessions, realm: "LiveKit Rooms"}
}

// Wrap authenticates requests via a session cookie, falling back to HTTP Basic
// Auth. On a successful Basic Auth login it issues a session cookie so the
// browser is not prompted for credentials again.
func (this *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			if username, ok := this.sessions.Validate(c.Value); ok {
				next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), username)))
				return
			}
		}

		username, password, ok := r.BasicAuth()
		if !ok || !this.users.Authenticate(r.Context(), username, password) {
			slog.Warn("authentication failed", "username", username, "remote_addr", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Basic realm="`+this.realm+`", charset="UTF-8"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, expiresAt, err := this.sessions.Create(username)
		if err != nil {
			http.Error(w, "failed to create session", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), username)))
	})
}

// HandleMe returns the identity of the currently authenticated user.
func (this *AuthMiddleware) HandleMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"identity": identity})
}

// HandleLogout invalidates the session and clears its cookie. It also returns a
// 401 so the browser drops any cached HTTP Basic Auth credentials.
func (this *AuthMiddleware) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		this.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("WWW-Authenticate", `Basic realm="`+this.realm+`", charset="UTF-8"`)
	http.Error(w, "logged out", http.StatusUnauthorized)
}
