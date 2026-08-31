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

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type IdentityResponse struct {
	Identity string `json:"identity"`
}

type AuthMiddleware struct {
	users    Authenticator
	sessions SessionManager
}

func NewAuthMiddleware(users Authenticator, sessions SessionManager) *AuthMiddleware {
	return &AuthMiddleware{users: users, sessions: sessions}
}

// Wrap authenticates requests via a session cookie. Unauthenticated requests
// receive a 401 without a Basic Auth challenge, so browsers never cache
// credentials and the client-side login form stays in control of the flow.
func (this *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			if username, ok := this.sessions.Validate(c.Value); ok {
				next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), username)))
				return
			}
		}

		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// HandleLogin validates the posted credentials and, on success, issues a
// session cookie.
func (this *AuthMiddleware) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var creds LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !this.users.Authenticate(r.Context(), creds.Username, creds.Password) {
		slog.Warn("authentication failed", "username", creds.Username, "remote_addr", r.RemoteAddr)
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	token, expiresAt, err := this.sessions.Create(creds.Username)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IdentityResponse{Identity: creds.Username})
}

// HandleMe returns the identity of the currently authenticated user.
func (this *AuthMiddleware) HandleMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IdentityResponse{Identity: identity})
}

// HandleLogout invalidates the session and clears its cookie.
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
	w.WriteHeader(http.StatusNoContent)
}
