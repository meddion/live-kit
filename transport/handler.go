package transport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/meddion/live-kit/api"
	"github.com/meddion/live-kit/auth"
)

type RoomService interface {
	AuthorizeJoin(context.Context, api.RoomJoinInput) (*api.RoomJoinResult, error)
	ListRooms(c context.Context, targetRooms ...string) (*api.RoomsResult, error)
}

type RoomHandler struct {
	liveKitMeetURL string
	srv            RoomService
}

func NewRoomHandler(srv RoomService, liveKitMeetURL string) *RoomHandler {
	return &RoomHandler{srv: srv, liveKitMeetURL: liveKitMeetURL}
}

type Room struct {
	Sid             string `json:"sid"`
	Name            string `json:"name"`
	CreationTime    int64  `json:"creation_time"`
	NumParticipants uint32 `json:"num_participants"`
	MaxParticipants uint32 `json:"max_participants"`
	ActiveRecording bool   `json:"active_recording"`
}

type RoomListResponse struct {
	Rooms []Room `json:"rooms"`
}

func (this *RoomHandler) HandleRoomList(w http.ResponseWriter, r *http.Request) {
	resp, err := this.srv.ListRooms(r.Context())
	if err != nil {
		slog.Error("failed to list rooms", "error", err)
		if errors.Is(err, api.ErrNotAuthorized) {
			http.Error(w, "User has no permissions to list meeting rooms.", http.StatusForbidden)
		} else {
			http.Error(w, "failed to list rooms", http.StatusInternalServerError)
		}
		return
	}

	rooms := make([]Room, 0, len(resp.Rooms))
	for _, room := range resp.Rooms {
		rooms = append(rooms, Room{
			Sid:             room.Sid,
			Name:            room.Name,
			CreationTime:    room.CreationTime,
			NumParticipants: room.NumParticipants,
			MaxParticipants: room.MaxParticipants,
			ActiveRecording: room.ActiveRecording,
		})
	}

	buf, err := json.Marshal(RoomListResponse{Rooms: rooms})
	if err != nil {
		slog.Error("failed to encode response payload", "error", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
}

func (this *RoomHandler) HandleRoomJoin(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	input := api.RoomJoinInput{
		Room:     r.PathValue("room"),
		Identity: identity,
	}
	res, err := this.srv.AuthorizeJoin(r.Context(), input)
	if err != nil {
		slog.Error("failed to authorize room join", "error", err)
		http.Error(w, "User has no permissions to create and join a meeting room.", http.StatusInternalServerError)
		return
	}

	resp := newJoinResponse(this.liveKitMeetURL, res.ServerURL, res.UserToken)
	buf, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to encode response payload", "error", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buf)
}

type RoomJoinResponse struct {
	ServerURL string `json:"server_url"`
	UserToken string `json:"user_token"`
	JoinURL   string `json:"join_url"`
}

func newJoinResponse(liveKitMeetURL, serverURL, userToken string) RoomJoinResponse {
	basePath, err := url.JoinPath(liveKitMeetURL, "custom")
	if err != nil {
		panic(err)
	}
	params := url.Values{}
	params.Set("liveKitUrl", serverURL)
	params.Set("token", userToken)

	return RoomJoinResponse{
		JoinURL:   basePath + "/?" + params.Encode(),
		ServerURL: serverURL,
		UserToken: userToken,
	}
}
