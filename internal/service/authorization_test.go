package service

import (
	"context"
	"encoding/base64"
	"testing"
	"time"
)

type authorizationRepositoryStub struct {
	AuthorizationRepository
	hashes  []string
	owner   string
	expires time.Time
	code    string
	now     time.Time
	id      string
	err     error
}

func (r *authorizationRepositoryStub) CreateAuthorization(_ context.Context, hash, owner string, expires time.Time) error {
	r.hashes = append(r.hashes, hash)
	r.owner, r.expires = owner, expires
	return r.err
}

func (r *authorizationRepositoryStub) CompleteAuthorization(_ context.Context, hash, code string, now time.Time) (string, error) {
	r.hashes = append(r.hashes, hash)
	r.code, r.now = code, now
	return r.id, r.err
}

func TestAuthorizationStateIsRandomHashedAndExpiring(t *testing.T) {
	repo := &authorizationRepositoryStub{id: "0123456789abcdef01234567"}
	auth := NewAuthorization(repo)
	before := time.Now()
	first, err := auth.Begin(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	second, err := auth.Begin(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil || len(nonce) != 32 || first == second || repo.hashes[0] != hashState(first) || repo.hashes[0] == first {
		t.Fatal("state must contain 256 random bits; only its digest is persisted")
	}
	if repo.owner != "42" || repo.expires.Before(before.Add(AuthorizationTTL)) || repo.expires.After(time.Now().Add(AuthorizationTTL)) {
		t.Fatal("session is not bound to its owner and expiry")
	}
	if _, err := auth.Complete(context.Background(), first, "spotify-code"); err != nil {
		t.Fatal(err)
	}
	if repo.hashes[2] != repo.hashes[0] || repo.code != "spotify-code" || repo.now.Before(before) {
		t.Fatal("callback did not use the same state digest or current-time expiry check")
	}
}

func TestAuthorizationRejectsInvalidInputAndUnknownState(t *testing.T) {
	repo := &authorizationRepositoryStub{}
	auth := NewAuthorization(repo)
	for _, id := range []string{"", "0", "-1", "not-a-user"} {
		if _, err := auth.Begin(context.Background(), id); err != ErrInvalidAuthorization {
			t.Fatalf("accepted user %q", id)
		}
	}
	for _, state := range []string{"", "abc", "not base64", base64.RawURLEncoding.EncodeToString(make([]byte, 31))} {
		if _, err := auth.Complete(context.Background(), state, "code"); err != ErrInvalidAuthorization {
			t.Fatalf("accepted state %q", state)
		}
	}
	if len(repo.hashes) != 0 {
		t.Fatal("invalid input reached storage")
	}
	valid := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	if _, err := auth.Complete(context.Background(), valid, ""); err != ErrInvalidAuthorization {
		t.Fatal("accepted empty code")
	}
	if _, err := auth.Complete(context.Background(), valid, "code"); err != ErrInvalidAuthorization {
		t.Fatal("accepted unknown state")
	}
}
