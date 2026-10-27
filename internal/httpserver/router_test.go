package httpserver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

type authStub struct {
	calls       int
	state, code string
	err         error
	panicValue  any
}

func (a *authStub) Complete(_ context.Context, state, code string) (string, error) {
	a.calls++
	a.state, a.code = state, code
	if a.panicValue != nil {
		panic(a.panicValue)
	}
	return "0123456789abcdef01234567", a.err
}

type healthStub struct{ err error }

func (h healthStub) Ping(context.Context) error { return h.err }

func TestCallbackContractAndNoCredentialLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, method, path string
		err                error
		panicValue         any
		status, calls      int
	}{
		{"valid", "GET", "/callback?code=secret-code&state=secret-state", nil, nil, 302, 1},
		{"trailing slash", "GET", "/callback/?code=secret-code&state=secret-state", nil, nil, 302, 1},
		{"no code", "GET", "/callback?state=secret-state", nil, nil, 400, 0},
		{"no state", "GET", "/callback?code=secret-code", nil, nil, 400, 0},
		{"duplicate state", "GET", "/callback?code=secret-code&state=a&state=b", nil, nil, 400, 0},
		{"duplicate code", "GET", "/callback?code=a&code=b&state=secret-state", nil, nil, 400, 0},
		{"malformed query", "GET", "/callback?code=secret-code&state=secret-state&x=%ZZ", nil, nil, 400, 0},
		{"denied", "GET", "/callback?error=access_denied&code=secret-code&state=secret-state", nil, nil, 200, 0},
		{"expired or replayed", "GET", "/callback?code=secret-code&state=secret-state", service.ErrInvalidAuthorization, nil, 400, 1},
		{"database failure", "GET", "/callback?code=secret-code&state=secret-state", errors.New("storage unavailable"), nil, 500, 1},
		{"panic", "GET", "/callback?code=secret-code&state=secret-state", nil, "unexpected failure", 500, 1},
		{"HEAD is read only", "HEAD", "/callback?code=secret-code&state=secret-state", nil, nil, 405, 0},
		{"HEAD slash", "HEAD", "/callback/?code=secret-code&state=secret-state", nil, nil, 405, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			auth := &authStub{err: test.err, panicValue: test.panicValue}
			router := NewRouter(auth, healthStub{}, "https://t.me/testbot?start=", slog.New(slog.NewJSONHandler(&logs, nil)))
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Authorization", "Bearer secret-header")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || auth.calls != test.calls {
				t.Fatalf("status=%d calls=%d body=%s", response.Code, auth.calls, response.Body)
			}
			if test.status == 302 && response.Header().Get("Location") != "https://t.me/testbot?start=0123456789abcdef01234567" {
				t.Fatal("incorrect Telegram return link")
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" || response.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing callback privacy headers")
			}
			for _, secret := range []string{"secret-code", "secret-state", "secret-header"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("credential %q leaked in logs", secret)
				}
			}
		})
	}
}

func TestHealthEndpoints(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		health := healthStub{}
		if unavailable {
			health.err = errors.New("database unavailable")
		}
		router := NewRouter(&authStub{}, health, "https://t.me/testbot?start=", slog.New(slog.DiscardHandler))
		for _, path := range []string{"/healthz", "/readyz"} {
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
				want := 200
				if path == "/readyz" && unavailable {
					want = 503
				}
				if response.Code != want {
					t.Fatalf("%s %s: got %d want %d", method, path, response.Code, want)
				}
			}
		}
	}
}
