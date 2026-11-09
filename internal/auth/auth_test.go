package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// sessionCookie signs a session with a and returns its cookie.
func sessionCookie(t *testing.T, a *Auth, cl claims) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	a.issue(c, cl)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == CookieName {
			return ck
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func claimsFor(subject string) jwt.RegisteredClaims { return jwt.RegisteredClaims{Subject: subject} }

func valid(a *Auth, ck *http.Cookie) (User, bool) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(ck)
	return a.Peek(c)
}

func TestSessionsFollowTheMode(t *testing.T) {
	const secret = "0123456789abcdef0123"
	pw := New(Options{Secret: secret, TTL: time.Hour, Password: "secret"})
	google := func(allowed ...string) *Auth {
		return New(Options{Secret: secret, TTL: time.Hour, Google: &Google{ClientID: "c", Allowed: allowed}})
	}

	pwCookie := sessionCookie(t, pw, claims{RegisteredClaims: claimsFor(passwordSubject)})
	if u, ok := valid(pw, pwCookie); !ok || u.ID() != "password" {
		t.Fatalf("password session: %v %v", u, ok)
	}
	if _, ok := valid(google("a@x.com"), pwCookie); ok {
		t.Fatal("a password session works in Google mode")
	}
	if _, ok := valid(New(Options{Secret: secret, TTL: time.Hour, Password: "changed"}), pwCookie); ok {
		t.Fatal("a session survived a password change")
	}

	g := google("A@x.com", "b@x.com")
	gCookie := sessionCookie(t, g, claims{RegisteredClaims: claimsFor("a@x.com"), Name: "A"})
	if u, ok := valid(g, gCookie); !ok || u.Email != "a@x.com" || u.Name != "A" {
		t.Fatalf("google session: %v %v", u, ok)
	}
	if _, ok := valid(google("b@x.com"), gCookie); ok {
		t.Fatal("a session outlived removing its email from ALLOWED_EMAILS")
	}
	if _, ok := valid(pw, gCookie); ok {
		t.Fatal("a Google session works in password mode")
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"/p/3/remote":         "/p/3/remote",
		"":                    "/",
		"https://evil.com":    "/",
		"//evil.com":          "/",
		"/\\evil.com":         "/",
		"/api/logout":         "/",
		"/login?next=%2Fp%2F": "/login?next=%2Fp%2F",
	} {
		if got := SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
