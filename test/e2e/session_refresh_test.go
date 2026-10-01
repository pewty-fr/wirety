//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestSessionSurvivesTokenRefresh keeps a dashboard session busy — bursts of
// concurrent API calls, like the frontend's polling — across several OIDC
// token lifetimes. Dex rotates refresh tokens, so a refresh done on a stale
// refresh token fails exactly like on most production IdPs.
//
// The session must stay valid throughout: every call must succeed, with the
// server refreshing the tokens transparently.
func TestSessionSurvivesTokenRefresh(t *testing.T) {
	const (
		tokenLifetime = 10 * time.Second
		duration      = 6 * tokenLifetime
		burstEvery    = 200 * time.Millisecond
		burstSize     = 15
	)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	st := setupStack(ctx, t, withDexTokenExpiry(tokenLifetime.String()))
	session, err := st.wiretySession(ctx, userEmail, userPassword)
	if err != nil {
		t.Fatalf("oidc login: %v", err)
	}

	client := st.inNetworkClient("")
	call := func() (int, string) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://server:8080/api/v1/users/me", nil)
		req.Header.Set("Cookie", "wirety_session="+session)
		resp, err := client.Do(req)
		if err != nil {
			return 0, err.Error()
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	var (
		mu       sync.Mutex
		calls    int
		failures []string
		wg       sync.WaitGroup
	)
	start := time.Now()
	tick := time.NewTicker(burstEvery)
	defer tick.Stop()
	for time.Since(start) < duration {
		<-tick.C
		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, body := call()
				mu.Lock()
				defer mu.Unlock()
				calls++
				if status != http.StatusOK {
					failures = append(failures, fmt.Sprintf("t+%s: %d %s", time.Since(start).Round(time.Millisecond), status, body))
				}
			}()
		}
	}
	wg.Wait()

	if len(failures) > 0 {
		shown := failures
		if len(shown) > 5 {
			shown = shown[:5]
		}
		t.Fatalf("%d/%d calls failed over %s of %s tokens; first failures:\n%v",
			len(failures), calls, duration, tokenLifetime, shown)
	}
	t.Logf("%d calls, all 200, over %s (%s tokens)", calls, duration, tokenLifetime)
}
