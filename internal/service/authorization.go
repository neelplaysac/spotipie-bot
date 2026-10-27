package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

const AuthorizationTTL = 10 * time.Minute

var ErrInvalidAuthorization = errors.New("authorization session is invalid, expired or already used")

// AuthorizationRepository persists a session across the bot and HTTP processes.
// Complete and Consume must atomically match and change a single document.
type AuthorizationRepository interface {
	CreateAuthorization(ctx context.Context, stateHash, telegramID string, expiresAt time.Time) error
	CompleteAuthorization(ctx context.Context, stateHash, code string, now time.Time) (string, error)
	ConsumeAuthorization(ctx context.Context, id, telegramID string, now time.Time) (*Code, error)
}

type Authorization struct {
	repo AuthorizationRepository
}

func NewAuthorization(repo AuthorizationRepository) *Authorization {
	return &Authorization{repo: repo}
}

func hashState(state string) string {
	hash := sha256.Sum256([]byte(state))
	return hex.EncodeToString(hash[:])
}

func (a *Authorization) Begin(ctx context.Context, telegramID string) (string, error) {
	id, err := strconv.ParseInt(telegramID, 10, 64)
	if err != nil || id <= 0 {
		return "", ErrInvalidAuthorization
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(nonce[:])
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.repo.CreateAuthorization(ctx, hashState(state), telegramID, time.Now().Add(AuthorizationTTL)); err != nil {
		return "", err
	}
	return state, nil
}

func (a *Authorization) Complete(ctx context.Context, state, code string) (string, error) {
	nonce, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil || len(nonce) != 32 || len(state) != 43 || code == "" || len(code) > 4096 {
		return "", ErrInvalidAuthorization
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	id, err := a.repo.CompleteAuthorization(ctx, hashState(state), code, time.Now())
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", ErrInvalidAuthorization
	}
	return id, nil
}

func (a *Authorization) Consume(ctx context.Context, id, telegramID string) (*Code, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return a.repo.ConsumeAuthorization(ctx, id, telegramID, time.Now())
}
