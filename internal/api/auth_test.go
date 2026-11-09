package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/audit"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"golang.org/x/oauth2"
)

// fakeGoogle plays Google's part of "Sign in with Google": the consent page
// sends the browser straight back, and the token endpoint checks PKCE.
type fakeGoogle struct {
	srv      *httptest.Server
	clientID string

	mu        sync.Mutex
	email     string
	verified  bool
	deny      bool // the person closes the consent screen
	nonce     string
	challenge string
}

func newFakeGoogle(t *testing.T, clientID, email string) *fakeGoogle {
	g := &fakeGoogle{clientID: clientID, email: email, verified: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		g.mu.Lock()
		g.nonce, g.challenge = q.Get("nonce"), q.Get("code_challenge")
		deny := g.deny
		g.mu.Unlock()
		if q.Get("client_id") != g.clientID || q.Get("code_challenge_method") != "S256" || q.Get("state") == "" {
			http.Error(w, "bad auth request", http.StatusBadRequest)
			return
		}
		back, _ := url.Parse(q.Get("redirect_uri"))
		v := url.Values{"state": {q.Get("state")}}
		if deny {
			v.Set("error", "access_denied")
		} else {
			v.Set("code", "good-code")
		}
		back.RawQuery = v.Encode()
		http.Redirect(w, r, back.String(), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		g.mu.Lock()
		defer g.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "good-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge ||
			r.Form.Get("client_secret") != "shh" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		claims, _ := json.Marshal(map[string]any{
			"iss": "https://accounts.google.com", "aud": g.clientID, "exp": time.Now().Add(time.Hour).Unix(),
			"nonce": g.nonce, "email": g.email, "email_verified": g.verified, "name": "Микола",
		})
		part := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600,
			"id_token": part([]byte(`{"alg":"RS256"}`)) + "." + part(claims) + ".sig",
		})
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func newGoogleEnv(t *testing.T, email string, allowed ...string) (*env, *fakeGoogle) {
	t.Helper()
	g := newFakeGoogle(t, "client-1", email)
	ga := &auth.Google{ClientID: "client-1", ClientSecret: "shh", Allowed: allowed,
		Endpoint: oauth2.Endpoint{AuthURL: g.srv.URL + "/auth", TokenURL: g.srv.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}}
	e := newEnvAuth(t, auth.Options{Google: ga})
	ga.RedirectURL = e.srv.URL + "/api/auth/google/callback"
	return e, g
}

// signIn follows the whole redirect chain and returns where the browser ended up.
func (e *env) signIn(next string) *url.URL {
	e.t.Helper()
	resp, err := e.client.Get(e.srv.URL + "/api/auth/google/start?next=" + url.QueryEscape(next))
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.Request.URL
}

func TestGoogleSignIn(t *testing.T) {
	e, _ := newGoogleEnv(t, "Kolya@Example.com", "kolya@example.com")
	if mode := e.do("GET", "/api/auth/config", nil, http.StatusOK)["mode"]; mode != "google" {
		t.Fatalf("mode = %v", mode)
	}
	e.do("POST", "/api/login", map[string]string{"password": ""}, http.StatusBadRequest)
	e.do("GET", "/api/session", nil, http.StatusUnauthorized)

	if end := e.signIn("/p/7/remote"); end.Path != "/p/7/remote" {
		t.Fatalf("after sign-in the browser is at %s, want /p/7/remote", end)
	}
	sess := e.do("GET", "/api/session", nil, http.StatusOK)
	user := sess["user"].(map[string]any)
	if user["email"] != "kolya@example.com" || user["name"] != "Микола" {
		t.Fatalf("session user = %v", user)
	}
	// Another site cannot be the destination.
	if end := e.signIn("//evil.example/x"); end.Host != strings.TrimPrefix(e.srv.URL, "http://") || end.Path != "/" {
		t.Fatalf("redirected off-site: %s", end)
	}
}

func TestGoogleSignInRefusals(t *testing.T) {
	e, g := newGoogleEnv(t, "stranger@example.com", "kolya@example.com")
	if end := e.signIn("/"); end.Path != "/login" || end.Query().Get("error") != "denied" {
		t.Fatalf("a stranger ended at %s", end)
	}
	e.do("GET", "/api/session", nil, http.StatusUnauthorized)

	g.mu.Lock()
	g.email, g.verified = "kolya@example.com", false
	g.mu.Unlock()
	if end := e.signIn("/"); end.Query().Get("error") != "failed" {
		t.Fatalf("an unverified email ended at %s", end)
	}

	g.mu.Lock()
	g.verified, g.deny = true, true
	g.mu.Unlock()
	if end := e.signIn("/"); end.Query().Get("error") != "cancelled" {
		t.Fatalf("closing Google's dialog ended at %s", end)
	}

	// A callback that this browser did not start (no state cookie) is refused.
	resp, err := e.client.Get(e.srv.URL + "/api/auth/google/callback?code=good-code&state=forged")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Request.URL.Query().Get("error") != "expired" {
		t.Fatalf("forged callback ended at %s", resp.Request.URL)
	}
	e.do("GET", "/api/session", nil, http.StatusUnauthorized)
}

// Every change is in the action log with who made it, and the log is also
// written to a file.
func TestActionLog(t *testing.T) {
	dir := t.TempDir()
	if err := audit.Open(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(audit.Close)

	e, _ := newGoogleEnv(t, "kolya@example.com", "kolya@example.com")
	e.signIn("/")
	pid := id(e.do("POST", "/api/projects", map[string]string{"title": "Журнал"}, http.StatusCreated))
	cid := id(e.do("POST", "/api/projects/"+itoa(pid)+"/categories", map[string]string{"title": "Хіти"}, http.StatusCreated))
	res := e.do("POST", "/api/categories/"+itoa(cid)+"/items", map[string]any{"urls": []string{"https://youtu.be/dQw4w9WgXcQ"}}, http.StatusCreated)
	iid := id(res["items"].([]any)[0].(map[string]any))
	e.do("PATCH", "/api/items/"+itoa(iid), map[string]any{"answer": "Rick Astley — Never Gonna Give You Up", "end": 20}, http.StatusOK)
	tid := id(e.do("POST", "/api/projects/"+itoa(pid)+"/teams", map[string]string{"name": "Бітли"}, http.StatusCreated))
	e.do("PUT", "/api/scores", map[string]any{"team_id": tid, "item_id": iid, "points": 1.5}, http.StatusOK)

	ws := e.dialLive(pid, "remote")
	readShow(t, ws)
	ws.WriteJSON(map[string]any{"type": "cmd", "action": "start"})
	waitPhase(t, ws, "category")

	matches, _ := filepath.Glob(filepath.Join(dir, "audit-*.log"))
	if len(matches) != 1 {
		t.Fatalf("log files: %v", matches)
	}
	data, _ := os.ReadFile(matches[0])
	log := string(data)
	for _, want := range []string{
		`who=kolya@example.com action=login`,
		`action=project.create`,
		`action=item.add`,
		`action=item.update`, `end: 15 → 20`,
		`action=score.set`, `team=Бітли`, `points="— → 1.5"`,
		`who=kolya@example.com action=show.start`, `phase=category`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
