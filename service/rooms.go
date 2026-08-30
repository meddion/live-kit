package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/meddion/live-kit/api"
)

// TokenValidTime needs to be valid for at least 5 minutes to allow the client to connect to the LiveKit server.
const TokenValidTime = 5 * time.Minute

const RoomEmptyTimeoutSec = 300
const RoomDepartureTimeoutSec = 300
const RoomMaxParticipants = 4

type PermissionChecker interface {
	Check(context.Context, ...api.Permission) (bool, error)
}

type Rooms struct {
	serverURL string
	pc        PermissionChecker
	roomAPI   *lksdk.RoomServiceClient
	tokenGen  TokenGenerator
}

func NewRooms(roomAPI *lksdk.RoomServiceClient, serverURL string, tokenGen TokenGenerator, pc PermissionChecker) *Rooms {
	if _, err := ValidateURL(serverURL); err != nil {
		panic(err)
	}

	return &Rooms{
		roomAPI:   roomAPI,
		serverURL: serverURL,
		tokenGen:  tokenGen,
		pc:        pc,
	}
}

// ListRooms returns a list of rooms from the LiveKit server.
// If targetRooms is provided, it filters the results to only include those rooms.
func (this *Rooms) ListRooms(c context.Context, targetRooms ...string) (*api.RoomsResult, error) {
	c, cancel := context.WithTimeout(c, 5*time.Second)
	defer cancel()

	// TODO: make a middleware for this
	authorized, err := this.pc.Check(c, api.PermViewRooms)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, api.ErrNotAuthorized
	}

	req := &livekit.ListRoomsRequest{Names: targetRooms}
	resp, err := this.roomAPI.ListRooms(c, req)
	if err != nil {
		return nil, err
	}

	respRooms := resp.GetRooms()
	rooms := make([]api.Room, 0, len(respRooms))
	for _, room := range respRooms {
		slog.Debug("room api struct", "room", room)

		rooms = append(rooms, api.Room{
			Sid:              room.Sid,
			Name:             room.Name,
			EmptyTimeout:     room.EmptyTimeout,
			DepartureTimeout: room.DepartureTimeout,
			MaxParticipants:  room.MaxParticipants,
			CreationTime:     room.CreationTime,
			CreationTimeMs:   room.CreationTimeMs,
			TurnPassword:     room.TurnPassword,
			Metadata:         room.Metadata,
			NumParticipants:  room.NumParticipants,
			NumPublishers:    room.NumPublishers,
			ActiveRecording:  room.ActiveRecording,
		})
	}

	return &api.RoomsResult{
		Rooms: rooms,
	}, nil
}

func (this *Rooms) AuthorizeJoin(c context.Context, input api.RoomJoinInput) (*api.RoomJoinResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	exist, err := this.roomExist(c, input.Room)
	if err != nil {
		return nil, fmt.Errorf("failed to check if room exists: %w", err)
	}

	// Skip this part if the room already exists.
	if !exist {
		if err := this.ensureRoom(c, input.Room); err != nil {
			return nil, fmt.Errorf("failed to ensure room exists: %w", err)
		}
	}

	// TODO: use AccessToken as a password to join or create the room.
	// For now, we just ignore it.
	_ = input.AccessToken

	token, err := this.tokenGen.NewToken(input.Room, input.Identity)
	if err != nil {
		return nil, fmt.Errorf("failed to generate a token: %w", err)
	}

	return &api.RoomJoinResult{
		ServerURL: this.serverURL,
		UserToken: token,
	}, nil
}

// ensureRoom creates the room if it does not exist yet.
func (this *Rooms) ensureRoom(c context.Context, room string) error {
	authorized, err := this.pc.Check(c, api.PermCreateRooms)
	if err != nil {
		return err
	}
	if !authorized {
		return api.ErrNotAuthorized
	}

	_, err = this.roomAPI.CreateRoom(c, &livekit.CreateRoomRequest{
		Name:             room,
		EmptyTimeout:     RoomEmptyTimeoutSec,
		DepartureTimeout: RoomDepartureTimeoutSec,
		MaxParticipants:  RoomMaxParticipants,
	})
	return err
}

func (this *Rooms) roomExist(c context.Context, room string) (bool, error) {
	authorized, err := this.pc.Check(c, api.PermJoinRooms)
	if err != nil {
		return false, err
	}
	if !authorized {
		return false, api.ErrNotAuthorized
	}

	req := &livekit.ListRoomsRequest{Names: []string{room}}
	resp, err := this.roomAPI.ListRooms(c, req)
	if err != nil {
		return false, err
	}

	return len(resp.GetRooms()) != 0, nil

}

type TokenGenerator struct {
	apiKey, apiSecret string
}

func NewTokenGenerator(apiKey, apiSecret string) TokenGenerator {
	return TokenGenerator{apiKey: apiKey, apiSecret: apiSecret}
}

func (this TokenGenerator) NewToken(room, identity string) (string, error) {
	at := auth.NewAccessToken(this.apiKey, this.apiSecret)
	at.SetVideoGrant(&auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}).
		SetIdentity(identity).
		SetValidFor(TokenValidTime)
	return at.ToJWT()
}

func ValidateURL(rawURL string) (*url.URL, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, errors.New("URL scheme must be http/https/ws/wss")
	}

	if u.Hostname() == "" {
		return nil, errors.New("URL must contain a host name")
	}

	return u, nil
}
