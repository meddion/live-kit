package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meddion/live-kit/db"
	"github.com/meddion/live-kit/service"
	"github.com/meddion/live-kit/transport"

	lksdk "github.com/livekit/server-sdk-go/v2"
)

const shutdownPeriod = 15 * time.Second

func main() {
	level := slog.LevelInfo
	if os.Getenv("APP_LOG_DEBUG") == "true" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := ParseConfigFromEnv()
	if err != nil {
		slog.Error("failed to parse the configuration", "error", err)
		os.Exit(1)
	}

	liveKitAPI, err := lksdk.NewLiveKitAPI(
		lksdk.WithURL(cfg.LiveKitPrivateServerURL),
		lksdk.WithAPIKey(cfg.APIKey, cfg.APISecret))
	if err != nil {
		slog.Error("creating live kit api", "error", err)
		os.Exit(1)
	}

	userStore, err := db.Open(rootCtx, cfg.DBFilePath)
	if err != nil {
		slog.Error("failed to open database", "path", cfg.DBFilePath, "error", err)
		os.Exit(1)
	}
	defer userStore.Close()
	if err := userStore.Seed(cfg.UsersFilePath); err != nil {
		slog.Error("failed to seed users", "path", cfg.UsersFilePath, "error", err)
		os.Exit(1)
	}

	tokenGen := service.NewTokenGenerator(cfg.APIKey, cfg.APISecret)
	pc := service.NewPermissionChecker(userStore)
	srv := service.NewRooms(liveKitAPI.Room(), cfg.LiveKitPublicServerURL, tokenGen, pc)
	handler := transport.NewRoomHandler(srv, cfg.LiveKitMeetURL)
	auth := transport.NewAuthMiddleware(userStore, service.NewSessionStore())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/rooms", handler.HandleRoomList)
	mux.HandleFunc("POST /api/v1/rooms/{room}/join", handler.HandleRoomJoin)
	mux.HandleFunc("GET /api/v1/me", auth.HandleMe)
	mux.HandleFunc("POST /api/v1/logout", auth.HandleLogout)
	mux.Handle("/", http.FileServer(http.Dir("frontend")))

	ongoingCtx, cancelOngoing := context.WithCancel(context.Background())
	defer cancelOngoing()
	server := &http.Server{
		Addr:    ":8080",
		Handler: auth.Wrap(mux),
		BaseContext: func(l net.Listener) context.Context {
			return ongoingCtx
		},
	}
	go func() {
		slog.Info("server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to stop http server", "error", err)
		}
	}()
	<-rootCtx.Done()
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownPeriod)
	defer cancel()
	err = server.Shutdown(shutdownCtx)
	cancelOngoing()
	if err != nil {
		slog.Error("gracefull shutdown failed")
		return
	}
	slog.Info("gracefully shutdown")
}

type Config struct {
	LiveKitPrivateServerURL string
	LiveKitPublicServerURL  string
	LiveKitMeetURL          string
	APIKey                  string
	APISecret               string
	UsersFilePath           string
	DBFilePath              string
}

func ParseConfigFromEnv() (Config, error) {
	serverURL, exist := os.LookupEnv("APP_PUBLIC_LIVEKIT_SERVER_URL")
	if !exist {
		slog.Error("APP_PUBLIC_LIVEKIT_SERVER_URL is not set")
		return Config{}, nil
	}
	privateServerURL := serverURL
	if privateURL, exist := os.LookupEnv("APP_PRIVATE_LIVEKIT_SERVER_URL"); exist {
		privateServerURL = privateURL
	} else {
		slog.Info("APP_PRIVATE_LIVEKIT_SERVER_URL not set, using default")
	}

	meetURL, exist := os.LookupEnv("APP_MEET_URL")
	if !exist {
		slog.Error("MEET_URL is not set")
		return Config{}, nil
	}

	apiKey, _ := os.LookupEnv("LIVEKIT_API_KEY")
	apiSecret, _ := os.LookupEnv("LIVEKIT_API_SECRET")
	if apiKey != "" && apiSecret != "" {
		slog.Info("using api key and secret from envirovment")
	} else {
		slog.Info("live-kit api key or secret not found: defaulting to the dev configuration")
		apiKey = "devkey"
		apiSecret = "secret"
	}

	usersFilePath, exist := os.LookupEnv("APP_USERS_FILE")
	if !exist {
		usersFilePath = "users.json"
	}

	dbFilePath, exist := os.LookupEnv("APP_DB_FILE")
	if !exist {
		dbFilePath = "main.db"
	}

	return Config{
		LiveKitPrivateServerURL: privateServerURL,
		LiveKitPublicServerURL:  serverURL,
		LiveKitMeetURL:          meetURL,
		APIKey:                  apiKey,
		APISecret:               apiSecret,
		UsersFilePath:           usersFilePath,
		DBFilePath:              dbFilePath,
	}, nil
}
