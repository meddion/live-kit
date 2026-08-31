package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/meddion/live-kit/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/meddion/live-kit/transport"
)

const (
	testUsername = "ivan"
	testPassword = "ivan-dev"
	testRoom     = "e2e-room"

	startupTimeout = 3 * time.Minute
)

type E2ESuite struct {
	suite.Suite
	ctx     context.Context
	stack   compose.ComposeStack
	client  *http.Client
	baseURL string
}

func TestE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}
	suite.Run(t, new(E2ESuite))
}

func (s *E2ESuite) SetupSuite() {
	s.ctx = context.Background()

	stack, err := compose.NewDockerComposeWith(compose.WithStackFiles("../docker-compose.yml"))
	require.NoError(s.T(), err)
	s.stack = stack

	err = stack.
		WithEnv(map[string]string{
			"ENV_FILE":                ".env.dev",
			"USERS_FILE_PATH":         "./users-dev.json",
			"LIVEKIT_SERVER_DEV_FLAG": "--dev",
		}).
		WaitForService("livekit-server", wait.ForListeningPort("7880/tcp").WithStartupTimeout(startupTimeout)).
		WaitForService("livekit-app", wait.ForHTTP("/").WithPort("8080/tcp").WithStartupTimeout(startupTimeout)).
		Up(s.ctx, compose.Wait(true))
	require.NoError(s.T(), err)

	container, err := stack.ServiceContainer(s.ctx, "livekit-app")
	require.NoError(s.T(), err)
	host, err := container.Host(s.ctx)
	require.NoError(s.T(), err)
	port, err := container.MappedPort(s.ctx, "8080")
	require.NoError(s.T(), err)

	jar, err := cookiejar.New(nil)
	require.NoError(s.T(), err)
	s.client = &http.Client{Jar: jar, Timeout: 15 * time.Second}
	s.baseURL = fmt.Sprintf("http://%s:%s", host, port.Port())
}

func (s *E2ESuite) TearDownSuite() {
	if s.stack != nil {
		_ = s.stack.Down(context.Background(), compose.RemoveOrphans(true), compose.RemoveImagesLocal, compose.RemoveVolumes(true))
	}
}

func (s *E2ESuite) TestEndToEndFlow() {
	s.Run("protected endpoints reject anonymous callers", s.requireProtectedEndpointsUnauthorized)
	s.Run("login issues a session", s.login)
	s.Run("me returns the authenticated identity", s.requireIdentity)
	s.Run("room is absent before joining", func() { s.Require().NotContains(s.listRooms(), testRoom) })
	s.Run("joining provisions a room and a token", s.joinRoom)
	s.Run("joined room is listed", func() { s.Require().Contains(s.listRooms(), testRoom) })
	s.Run("logout clears the session", s.logout)
	s.Run("protected endpoints reject after logout", s.requireProtectedEndpointsUnauthorized)
}

// TestPermissionChecks verifies that users lacking a permission are denied the
// resource it guards, even though they can authenticate.
func (s *E2ESuite) TestPermissionChecks() {
	cases := []struct {
		name     string
		username string
		password string
		method   string
		path     string
		wantCode int
	}{
		{"jack lacks CreateRooms and cannot create a room", "jack", "jack-dev", http.MethodPost, "/api/v1/rooms/jack-room/join", http.StatusInternalServerError},
		{"tony lacks ViewRooms and cannot list rooms", "tony", "tony-dev", http.MethodGet, "/api/v1/rooms", http.StatusForbidden},
	}
	for _, c := range cases {
		s.Run(c.name, func() {
			s.loginAs(c.username, c.password)
			resp := s.do(c.method, c.path, nil)
			resp.Body.Close()
			s.Require().Equal(c.wantCode, resp.StatusCode)
			s.logout()
		})
	}
}

func (s *E2ESuite) requireProtectedEndpointsUnauthorized() {
	endpoints := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/rooms"},
		{http.MethodPost, "/api/v1/rooms/" + testRoom + "/join"},
		{http.MethodPost, "/api/v1/logout"},
	}
	for _, e := range endpoints {
		resp := s.do(e.method, e.path, nil)
		resp.Body.Close()
		s.Require().Equalf(http.StatusUnauthorized, resp.StatusCode, "%s %s", e.method, e.path)
	}
}

// login will store the session cookie in the client jar, so subsequent requests will be authenticated.
func (s *E2ESuite) login() {
	s.loginAs(testUsername, testPassword)
}

func (s *E2ESuite) loginAs(username, password string) {
	resp := s.do(http.MethodPost, "/api/v1/login", transport.LoginRequest{
		Username: username,
		Password: password,
	})
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var out transport.IdentityResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&out))
	s.Require().Equal(username, out.Identity)
}

func (s *E2ESuite) requireIdentity() {
	resp := s.do(http.MethodGet, "/api/v1/me", nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var out transport.IdentityResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&out))
	s.Require().Equal(testUsername, out.Identity)
}

func (s *E2ESuite) joinRoom() {
	resp := s.do(http.MethodPost, "/api/v1/rooms/"+testRoom+"/join", nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var out transport.RoomJoinResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&out))
	s.Require().NotEmpty(out.ServerURL)
	s.Require().NotEmpty(out.UserToken)
	s.Require().Contains(out.JoinURL, out.UserToken)

	_, err := service.ValidateURL(out.JoinURL)
	s.Require().NoError(err)
}

func (s *E2ESuite) logout() {
	resp := s.do(http.MethodPost, "/api/v1/logout", nil)
	resp.Body.Close()
	s.Require().Equal(http.StatusNoContent, resp.StatusCode)
}

func (s *E2ESuite) listRooms() []string {
	resp := s.do(http.MethodGet, "/api/v1/rooms", nil)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
	var out transport.RoomListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&out))

	names := make([]string, 0, len(out.Rooms))
	for _, r := range out.Rooms {
		names = append(names, r.Name)
	}
	return names
}

func (s *E2ESuite) do(method, path string, body any) *http.Response {
	var reader *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(s.ctx, method, s.baseURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	return resp
}
