package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type accountRepositoryStub struct {
	AccountRepository
	code  *Code
	user  *User
	fm    *LastFMUser
	err   error
	calls []string
	name  string
	token string
}

func (r *accountRepositoryStub) FetchCode(context.Context, string) (*Code, error) {
	r.calls = append(r.calls, "fetch-code")
	return r.code, r.err
}
func (r *accountRepositoryStub) DeleteCode(context.Context, string) error {
	r.calls = append(r.calls, "delete-code")
	return nil
}
func (r *accountRepositoryStub) FetchData(context.Context, string) (*User, error) {
	r.calls = append(r.calls, "fetch-user")
	return r.user, r.err
}
func (r *accountRepositoryStub) AddUser(_ context.Context, _, token string) error {
	r.calls = append(r.calls, "add-user")
	r.token = token
	return r.err
}
func (r *accountRepositoryStub) UpdateData(_ context.Context, _, name string) error {
	r.name = name
	return r.err
}
func (r *accountRepositoryStub) GetLastFMUser(context.Context, string) (*LastFMUser, error) {
	r.calls = append(r.calls, "fetch-lastfm")
	return r.fm, r.err
}
func (r *accountRepositoryStub) AddLastFMUser(_ context.Context, _, name string) error {
	r.name = name
	return r.err
}

type authorizationStub struct {
	token string
	err   error
	code  string
}

func (s *authorizationStub) GetAuthUrl() string { return "https://example.invalid/authorize" }
func (s *authorizationStub) GetAccessToken(_ context.Context, code string) (string, error) {
	s.code = code
	return s.token, s.err
}

func TestCompleteSpotifyRegistration(t *testing.T) {
	for _, test := range []struct {
		name   string
		codeID string
		code   *Code
		user   *User
		token  string
		status AccountStatus
		calls  []string
	}{
		{"invalid code", "invalid", nil, nil, "", AccountInvalidCode, nil},
		{"missing code", "0123456789abcdef01234567", nil, nil, "", AccountMissingCode, []string{"fetch-code", "delete-code"}},
		{"already linked", "0123456789abcdef01234567", &Code{AuthCode: "code"}, &User{}, "", AccountAlreadyLinked, []string{"fetch-code", "delete-code", "fetch-user"}},
		{"auth rejected", "0123456789abcdef01234567", &Code{AuthCode: "code"}, nil, "error", AccountAuthFailed, []string{"fetch-code", "delete-code", "fetch-user"}},
		{"linked", "0123456789abcdef01234567", &Code{AuthCode: "code"}, nil, "refresh-token", AccountUpdated, []string{"fetch-code", "delete-code", "fetch-user", "add-user"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &accountRepositoryStub{code: test.code, user: test.user}
			spotify := &authorizationStub{token: test.token}
			status, err := NewAccounts(repo, spotify).CompleteSpotifyRegistration(context.Background(), test.codeID, "42")
			if err != nil || status != test.status || !reflect.DeepEqual(repo.calls, test.calls) {
				t.Fatalf("status=%q err=%v calls=%v; want %q %v", status, err, repo.calls, test.status, test.calls)
			}
			if status == AccountUpdated && (repo.token != "refresh-token" || spotify.code != "code") {
				t.Fatal("authorization code or refresh token changed across the service boundary")
			}
		})
	}
}

func TestRegistrationPreservesCodeLookupFailure(t *testing.T) {
	repo := &accountRepositoryStub{err: errors.New("database unavailable")}
	status, err := NewAccounts(repo, nil).CompleteSpotifyRegistration(context.Background(), "0123456789abcdef01234567", "42")
	if err != nil || status != AccountInvalidCode || !reflect.DeepEqual(repo.calls, []string{"fetch-code", "delete-code"}) {
		t.Fatal("code lookup failure no longer follows the original login response")
	}
}

func TestAccountValidationAndNameUpdates(t *testing.T) {
	for _, name := range []string{"", "   ", "/command", "sixteen-chars!!!x"} {
		repo := &accountRepositoryStub{user: &User{}}
		status, err := NewAccounts(repo, nil).SetSpotifyName(context.Background(), "42", name)
		if err != nil || status != AccountInvalidInput || len(repo.calls) != 0 {
			t.Fatalf("invalid display name %q reached storage", name)
		}
	}
	repo := &accountRepositoryStub{user: &User{}}
	status, err := NewAccounts(repo, nil).SetSpotifyName(context.Background(), "42", "  नमस्ते  ")
	if err != nil || status != AccountUpdated || repo.name != "नमस्ते" {
		t.Fatal("Unicode display names or whitespace trimming changed")
	}
	for _, name := range []string{"invalid name", "नमस्ते", "/user"} {
		repo := &accountRepositoryStub{}
		status, err := NewAccounts(repo, nil).LinkLastFM(context.Background(), "42", name)
		if err != nil || status != AccountInvalidInput || len(repo.calls) != 0 {
			t.Fatalf("invalid Last.fm username %q reached storage", name)
		}
	}
	repo = &accountRepositoryStub{}
	status, err = NewAccounts(repo, nil).LinkLastFM(context.Background(), "42", " fm_user-1 ")
	if err != nil || status != AccountUpdated || repo.name != "fm_user-1" {
		t.Fatal("Last.fm linking changed")
	}
}
