package service

import (
	"context"
	"encoding/hex"
	"strings"
)

type User struct {
	Username string
	Token    string
	Style    string
	TgID     string
}

type LastFMUser struct {
	FMUsername string
	Name       string
	TgID       string
	Counter    string
}

type Code struct {
	AuthCode       string
	TelegramUserID string
}

type Count struct {
	ID    string
	Count int64
}

type AccountRepository interface {
	FetchCode(context.Context, string) (*Code, error)
	DeleteCode(context.Context, string) error
	FetchData(context.Context, string) (*User, error)
	AddUser(context.Context, string, string) error
	UpdateData(context.Context, string, string) error
	UpdateStyle(context.Context, string, string) error
	DeleteData(context.Context, string) error
	GetLastFMUser(context.Context, string) (*LastFMUser, error)
	AddLastFMUser(context.Context, string, string) error
	UpdateLastFMData(context.Context, string, string) error
	RemoveLastFMUser(context.Context, string) error
	ToggleCounter(context.Context, string, string) error
	CountAll(context.Context) (int64, error)
	CountAllLastFM(context.Context) (int64, error)
	AggregateUsers(context.Context) ([]Count, error)
	AggregateLastFMUsers(context.Context) ([]Count, error)
}

type SpotifyAuthorization interface {
	GetAuthUrl() string
	GetAccessToken(context.Context, string) (string, error)
}

type AccountStatus string

const (
	AccountUpdated       AccountStatus = "updated"
	AccountInvalidInput  AccountStatus = "invalid_input"
	AccountNotRegistered AccountStatus = "not_registered"
	AccountAlreadyLinked AccountStatus = "already_linked"
	AccountInvalidCode   AccountStatus = "invalid_code"
	AccountMissingCode   AccountStatus = "missing_code"
	AccountAuthFailed    AccountStatus = "auth_failed"
)

type Accounts struct {
	repo    AccountRepository
	spotify SpotifyAuthorization
}

func NewAccounts(repo AccountRepository, spotify SpotifyAuthorization) *Accounts {
	return &Accounts{repo: repo, spotify: spotify}
}

func (a *Accounts) SpotifyAuthURL() string { return a.spotify.GetAuthUrl() }

func (a *Accounts) CompleteSpotifyRegistration(ctx context.Context, id, tgID string) (AccountStatus, error) {
	if _, err := hex.DecodeString(id); len(id) != 24 || err != nil {
		return AccountInvalidCode, nil
	}
	code, err := a.repo.FetchCode(ctx, id)
	_ = a.repo.DeleteCode(ctx, id)
	if err != nil {
		return AccountInvalidCode, nil
	}
	if code == nil {
		return AccountMissingCode, nil
	}
	existing, err := a.repo.FetchData(ctx, tgID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return AccountAlreadyLinked, nil
	}
	token, err := a.spotify.GetAccessToken(ctx, code.AuthCode)
	if err != nil {
		return "", err
	}
	if token == "error" {
		return AccountAuthFailed, nil
	}
	if err := a.repo.AddUser(ctx, tgID, token); err != nil {
		return "", err
	}
	return AccountUpdated, nil
}

func validDisplayName(text string) bool {
	return text != "" && len([]rune(text)) <= 15 && !strings.HasPrefix(text, "/")
}

func (a *Accounts) SetSpotifyName(ctx context.Context, tgID, text string) (AccountStatus, error) {
	text = strings.TrimSpace(text)
	if !validDisplayName(text) {
		return AccountInvalidInput, nil
	}
	user, err := a.repo.FetchData(ctx, tgID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return AccountNotRegistered, nil
	}
	return AccountUpdated, a.repo.UpdateData(ctx, tgID, text)
}

func (a *Accounts) UnlinkSpotify(ctx context.Context, tgID string) (bool, error) {
	user, err := a.repo.FetchData(ctx, tgID)
	if err != nil || user == nil {
		return false, err
	}
	return true, a.repo.DeleteData(ctx, tgID)
}

func validFMUsername(text string) bool {
	for _, r := range text {
		if r != '-' && r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func (a *Accounts) LinkLastFM(ctx context.Context, tgID, text string) (AccountStatus, error) {
	text = strings.TrimSpace(text)
	if !validDisplayName(text) || !validFMUsername(text) {
		return AccountInvalidInput, nil
	}
	user, err := a.repo.GetLastFMUser(ctx, tgID)
	if err != nil {
		return "", err
	}
	if user != nil {
		return AccountAlreadyLinked, nil
	}
	return AccountUpdated, a.repo.AddLastFMUser(ctx, tgID, text)
}

func (a *Accounts) SetLastFMName(ctx context.Context, tgID, text string) (AccountStatus, error) {
	text = strings.TrimSpace(text)
	if !validDisplayName(text) {
		return AccountInvalidInput, nil
	}
	user, err := a.repo.GetLastFMUser(ctx, tgID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return AccountNotRegistered, nil
	}
	return AccountUpdated, a.repo.UpdateLastFMData(ctx, tgID, text)
}

func (a *Accounts) UnlinkLastFM(ctx context.Context, tgID string) (bool, error) {
	user, err := a.repo.GetLastFMUser(ctx, tgID)
	if err != nil || user == nil {
		return false, err
	}
	return true, a.repo.RemoveLastFMUser(ctx, tgID)
}

type Settings struct {
	Spotify *User
	LastFM  *LastFMUser
}

func (a *Accounts) Settings(ctx context.Context, tgID string) (*Settings, error) {
	user, err := a.repo.FetchData(ctx, tgID)
	if err != nil {
		return nil, err
	}
	fmUser, err := a.repo.GetLastFMUser(ctx, tgID)
	if err != nil {
		return nil, err
	}
	if fmUser != nil && fmUser.Counter == "" {
		_ = a.repo.ToggleCounter(ctx, tgID, "on")
		fmUser.Counter = "on"
	}
	return &Settings{Spotify: user, LastFM: fmUser}, nil
}

func (a *Accounts) UpdateStyle(ctx context.Context, tgID, value string) error {
	return a.repo.UpdateStyle(ctx, tgID, value)
}

func (a *Accounts) ToggleCounter(ctx context.Context, tgID, value string) error {
	return a.repo.ToggleCounter(ctx, tgID, value)
}

func (a *Accounts) CountSpotify(ctx context.Context) (int64, error) {
	return a.repo.CountAll(ctx)
}

type Stats struct {
	SpotifyUsers int64
	LastFMUsers  int64
	Blur         int64
	CounterOn    int64
	CounterOff   int64
}

func (a *Accounts) Stats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}
	var err error
	stats.SpotifyUsers, err = a.repo.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	stats.LastFMUsers, err = a.repo.CountAllLastFM(ctx)
	if err != nil {
		return nil, err
	}
	users, err := a.repo.AggregateUsers(ctx)
	if err != nil {
		return nil, err
	}
	fmUsers, err := a.repo.AggregateLastFMUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, count := range users {
		if count.ID == "blur" {
			stats.Blur += count.Count
		}
	}
	for _, count := range fmUsers {
		switch count.ID {
		case "on":
			stats.CounterOn += count.Count
		case "off":
			stats.CounterOff += count.Count
		}
	}
	return stats, nil
}
