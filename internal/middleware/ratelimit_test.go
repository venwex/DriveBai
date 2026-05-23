package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/drivebai/backend/internal/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// TestRateLimit_AllowsUnderLimit verifies that requests below the cap pass through.
func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(5, time.Minute)
	defer rl.Stop()

	handler := middleware.RateLimit(rl)(okHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

// TestRateLimit_BlocksOverLimit verifies that the (n+1)th request in the same
// window receives 429 Too Many Requests.
func TestRateLimit_BlocksOverLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	handler := middleware.RateLimit(rl)(okHandler())
	ip := "10.0.0.1:1234"

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// The 4th request must be throttled.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after limit exceeded, got %d", rr.Code)
	}
}

// TestRateLimit_DifferentIPsAreIndependent verifies per-IP isolation.
func TestRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	rl := middleware.NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handler := middleware.RateLimit(rl)(okHandler())

	for _, ip := range []string{"1.1.1.1:80", "2.2.2.2:80", "3.3.3.3:80"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("ip %s: expected 200 for first request, got %d", ip, rr.Code)
		}
	}
}

// TestRateLimit_WindowReset verifies that a new window resets the counter.
func TestRateLimit_WindowReset(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 50*time.Millisecond)
	defer rl.Stop()

	handler := middleware.RateLimit(rl)(okHandler())
	ip := "172.16.0.1:8080"

	do := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	// First request succeeds.
	if code := do(); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	// Second request in same window is throttled.
	if code := do(); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}

	// Wait for the window to expire, then try again.
	time.Sleep(60 * time.Millisecond)
	if code := do(); code != http.StatusOK {
		t.Errorf("expected 200 after window reset, got %d", code)
	}
}
