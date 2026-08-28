package api

import (
	"errors"
)

type RoomJoinInput struct {
	Room        string
	Identity    string
	AccessToken string
}

func (req RoomJoinInput) Validate() error {
	if len(req.Room) < 1 || len([]rune(req.Room)) > 32 {
		return errors.New("room must be set and it's length should not exceed 32 symbols")
	}
	if len(req.Identity) < 1 || len([]rune(req.Identity)) > 32 {
		return errors.New("identity must be set and it's length should not exceed 32 symbols")
	}

	return nil
}

type RoomJoinResult struct {
	ServerURL string
	UserToken string
}

type Room struct {
	Sid              string
	Name             string
	EmptyTimeout     uint32
	DepartureTimeout uint32
	MaxParticipants  uint32
	CreationTime     int64
	CreationTimeMs   int64
	TurnPassword     string
	Metadata         string
	NumParticipants  uint32
	NumPublishers    uint32
	ActiveRecording  bool
}

type RoomsResult struct {
	Rooms []Room
}

var (
	ErrNotAuthorized = errors.New("user not authorized to perform this action")
)

type Permission string

const (
	PermViewRooms   Permission = "ViewRooms"
	PermJoinRooms   Permission = "JoinRooms"
	PermCreateRooms Permission = "CreateRooms"
	PermGodAlmighty Permission = "GodAlmighty"
)
