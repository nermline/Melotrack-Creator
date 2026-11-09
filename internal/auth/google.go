package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

// Google signs organisers in with their Google account (OpenID Connect,
// authorization code flow with PKCE). Only emails in Allowed get a session.
type Google struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string          // PUBLIC_URL/api/auth/google/callback
	Allowed      []string        // lower-case emails
	Endpoint     oauth2.Endpoint // zero value = Google's (tests point it elsewhere)

	allow map[string]bool
}

var googleEndpoint = oauth2.Endpoint{
	AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL:  "https://oauth2.googleapis.com/token",
	AuthStyle: oauth2.AuthStyleInParams,
}

func (g *Google) config() *oauth2.Config {
	ep := g.Endpoint
	if ep.AuthURL == "" {
		ep = googleEndpoint
	}
	return &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		RedirectURL:  g.RedirectURL,
		Endpoint:     ep,
		Scopes:       []string{"openid", "email", "profile"},
	}
}

func (g *Google) init() {
	g.allow = map[string]bool{}
	for _, e := range g.Allowed {
		g.allow[strings.ToLower(strings.TrimSpace(e))] = true
	}
}

func (g *Google) allowed(email string) bool {
	return email != "" && g.allow[strings.ToLower(email)]
}

const stateCookie = "mt_oauth"
const statePath = "/api/auth/google/"

// oauthState travels in a short-lived signed cookie between the redirect to
// Google and the way back.
type oauthState struct {
	jwt.RegisteredClaims
	State    string `json:"st"`
	Verifier string `json:"cv"`
	Nonce    string `json:"nn"`
	Next     string `json:"nx"`
}

func randomString() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// GoogleStart sends the browser to Google's account chooser.
func (a *Auth) GoogleStart(c *gin.Context) {
	st := oauthState{
		State: randomString(), Verifier: oauth2.GenerateVerifier(), Nonce: randomString(),
		Next: SafeNext(c.Query("next")),
	}
	st.ExpiresAt = jwt.NewNumericDate(time.Now().Add(10 * time.Minute))
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, st).SignedString(a.key)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=failed")
		return
	}
	a.setCookie(c, stateCookie, statePath, signed, 600)
	url := a.google.config().AuthCodeURL(st.State,
		oauth2.S256ChallengeOption(st.Verifier),
		oauth2.SetAuthURLParam("nonce", st.Nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"))
	c.Redirect(http.StatusFound, url)
}

// Sign-in errors, as codes for the login page.
var (
	ErrCancelled  = errors.New("cancelled") // the person closed Google's dialog
	ErrExpired    = errors.New("expired")   // state cookie missing or stale: start again
	ErrNotAllowed = errors.New("denied")    // a Google account that is not on the list
	ErrFailed     = errors.New("failed")    // anything else
)

// GoogleCallback finishes the sign-in. On success the session cookie is set
// and next is where the person was going. The user is returned on failure
// too when Google told us who it was (for the log).
func (a *Auth) GoogleCallback(c *gin.Context) (u User, next string, err error) {
	raw, _ := c.Cookie(stateCookie)
	a.setCookie(c, stateCookie, statePath, "", -1)
	if e := c.Query("error"); e != "" {
		if e == "access_denied" {
			return u, "", ErrCancelled
		}
		return u, "", fmt.Errorf("%w: google: %s", ErrFailed, e)
	}
	st := &oauthState{}
	if _, perr := jwt.ParseWithClaims(raw, st, func(*jwt.Token) (any, error) { return a.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})); perr != nil || raw == "" {
		return u, "", ErrExpired
	}
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(st.State)) != 1 {
		return u, "", ErrExpired
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	tok, xerr := a.google.config().Exchange(ctx, c.Query("code"), oauth2.VerifierOption(st.Verifier))
	if xerr != nil {
		return u, "", fmt.Errorf("%w: exchange: %v", ErrFailed, xerr)
	}
	idToken, _ := tok.Extra("id_token").(string)
	id, perr := parseIDToken(idToken)
	if perr != nil {
		return u, "", fmt.Errorf("%w: %v", ErrFailed, perr)
	}
	if verr := id.check(a.google.ClientID, st.Nonce, time.Now()); verr != nil {
		return u, "", fmt.Errorf("%w: %v", ErrFailed, verr)
	}
	u = User{Email: strings.ToLower(id.Email), Name: id.Name, Picture: id.Picture}
	if !a.google.allowed(u.Email) {
		return u, "", ErrNotAllowed
	}
	cl := claims{Name: u.Name, Picture: u.Picture}
	cl.Subject = u.Email
	a.issue(c, cl)
	return u, st.Next, nil
}

// idClaims is the part of Google's ID token we use.
type idClaims struct {
	Issuer        string   `json:"iss"`
	Audience      audience `json:"aud"`
	Expires       int64    `json:"exp"`
	Nonce         string   `json:"nonce"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	Picture       string   `json:"picture"`
}

// audience is a string or a list of strings.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*a = audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

// parseIDToken reads the token's claims. The signature is not checked: the
// token comes straight from Google's token endpoint over TLS in exchange for
// our client secret, which OpenID Connect accepts in place of a signature
// check (Core 1.0, section 3.1.3.7).
func parseIDToken(raw string) (*idClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("no id_token in the response")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, fmt.Errorf("id_token: %w", err)
	}
	var id idClaims
	if err := json.Unmarshal(payload, &id); err != nil {
		return nil, fmt.Errorf("id_token: %w", err)
	}
	return &id, nil
}

func (id *idClaims) check(clientID, nonce string, now time.Time) error {
	if id.Issuer != "https://accounts.google.com" && id.Issuer != "accounts.google.com" {
		return fmt.Errorf("id_token from %q", id.Issuer)
	}
	ok := false
	for _, aud := range id.Audience {
		ok = ok || aud == clientID
	}
	if !ok {
		return errors.New("id_token for another client")
	}
	if now.Unix() > id.Expires+60 {
		return errors.New("id_token expired")
	}
	if subtle.ConstantTimeCompare([]byte(id.Nonce), []byte(nonce)) != 1 {
		return errors.New("id_token nonce mismatch")
	}
	if id.Email == "" || !id.EmailVerified {
		return errors.New("the Google account has no verified email")
	}
	return nil
}
