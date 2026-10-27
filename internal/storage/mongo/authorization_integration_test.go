package mongostore_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/httpserver"
	"github.com/neelplaysac/spotipie-go/internal/service"
	mongostore "github.com/neelplaysac/spotipie-go/internal/storage/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func testDatabase(t *testing.T) *mongostore.Database {
	t.Helper()
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("set MONGODB_TEST_URI to a disposable local MongoDB to run integration tests")
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "mongodb" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		t.Fatal("MONGODB_TEST_URI must point to a disposable loopback MongoDB")
	}
	name := "spotipie_test_" + bson.NewObjectID().Hex()
	db, err := mongostore.New(&config.Config{MongoURI: uri, MongoDatabase: name}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := mongo.Connect(options.Client().ApplyURI(uri))
		if err != nil {
			t.Error(err)
			return
		}
		defer client.Disconnect(ctx)
		if err := client.Database(name).Drop(ctx); err != nil {
			t.Error(err)
		}
	})
	return db
}

func TestMongoAuthorizationIsAtomicAndOwnerBound(t *testing.T) {
	db := testDatabase(t)
	auth := service.NewAuthorization(db)
	ctx := context.Background()
	state, err := auth.Begin(ctx, "42")
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for range 16 {
		wg.Go(func() {
			id, err := auth.Complete(ctx, state, "spotify-code")
			if err == nil {
				wins.Add(1)
				ids <- id
			} else if !errors.Is(err, service.ErrInvalidAuthorization) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("callback accepted %d times", wins.Load())
	}
	id := <-ids
	if code, err := auth.Consume(ctx, id, "99"); err != nil || code != nil {
		t.Fatal("wrong owner obtained the code")
	}
	wins.Store(0)
	for range 16 {
		wg.Go(func() {
			code, err := auth.Consume(ctx, id, "42")
			if err != nil {
				t.Error(err)
				return
			}
			if code != nil {
				wins.Add(1)
				if code.AuthCode != "spotify-code" || code.TelegramUserID != "42" {
					t.Error("code or owner changed")
				}
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("code consumed %d times", wins.Load())
	}
}

func TestMongoExpiryIsEnforcedWithoutWaitingForTTLDeletion(t *testing.T) {
	db := testDatabase(t)
	ctx := context.Background()
	now := time.Now()
	if err := db.CreateAuthorization(ctx, "expired", "42", now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if id, err := db.CompleteAuthorization(ctx, "expired", "code", now); err != nil || id != "" {
		t.Fatal("expired state was accepted")
	}
	if err := db.CreateAuthorization(ctx, "valid", "42", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	id, err := db.CompleteAuthorization(ctx, "valid", "code", now)
	if err != nil || id == "" {
		t.Fatalf("valid callback failed: %v", err)
	}
	if code, err := db.ConsumeAuthorization(ctx, id, "42", now.Add(2*time.Minute)); err != nil || code != nil {
		t.Fatal("expired deep link was accepted")
	}
}

type spotifyStub struct{}

func (spotifyStub) AuthURL(state string) string {
	return "https://accounts.spotify.com/authorize?state=" + url.QueryEscape(state)
}
func (spotifyStub) GetAccessToken(_ context.Context, code string) (string, error) {
	return "refresh-" + code, nil
}

func TestRegistrationThroughHTTPAndMongo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testDatabase(t)
	auth := service.NewAuthorization(db)
	accounts := service.NewAccounts(db, spotifyStub{}, auth)
	router := httpserver.NewRouter(auth, db, "https://t.me/testbot?start=", slog.New(slog.DiscardHandler))
	ctx := context.Background()
	// Two users on the same IP must receive distinct sessions and tokens.
	for _, user := range []string{"42", "99"} {
		link, err := accounts.SpotifyAuthURL(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(link)
		callbackURL := "/callback?code=code-" + user + "&state=" + url.QueryEscape(u.Query().Get("state"))
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, callbackURL, nil)
		request.RemoteAddr = "192.0.2.1:12345"
		router.ServeHTTP(response, request)
		if response.Code != 302 {
			t.Fatalf("callback: status=%d body=%s", response.Code, response.Body)
		}
		redirect, _ := url.Parse(response.Header().Get("Location"))
		id := redirect.Query().Get("start")
		if status, err := accounts.CompleteSpotifyRegistration(ctx, id, "123"); err != nil || status != service.AccountMissingCode {
			t.Fatal("a different Telegram user could use the link")
		}
		if status, err := accounts.CompleteSpotifyRegistration(ctx, id, user); err != nil || status != service.AccountUpdated {
			t.Fatalf("registration: %s %v", status, err)
		}
		saved, err := db.FetchData(ctx, user)
		if err != nil || saved == nil || saved.Token != "refresh-code-"+user || saved.Style != "blur" || saved.Username != "User" {
			t.Fatal("legacy user schema or token changed")
		}
		if status, err := accounts.CompleteSpotifyRegistration(ctx, id, user); err != nil || status != service.AccountMissingCode {
			t.Fatal("deep link could be replayed")
		}
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, callbackURL, nil))
		if response.Code != 400 {
			t.Fatal("callback could be replayed")
		}
	}
}
