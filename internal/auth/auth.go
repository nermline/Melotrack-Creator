// Package auth protects the organiser side. Organisers sign in either with one
// shared password or with their Google account (only listed emails get in).
//
// A successful sign-in sets a signed JWT in an HttpOnly cookie. Because it is
// a cookie, <video>, <img> and WebSocket requests are authenticated
// automatically, and the session slides forward while in use, so a
// presentation screen left running for hours keeps working. Changing
// APP_PASSWORD or switching the mode logs everyone out.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const CookieName = "mt_session"

// Mode is how organisers sign in.
type Mode string

const (
	ModePassword Mode = "password"
	ModeGoogle   Mode = "google"
)

// passwordSubject is the session subject of the shared password.
const passwordSubject = "organiser"

// User is who a session belongs to.
type User struct {
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
	Picture string `json:"picture,omitempty"`
}

// ID names the user in the action log: the email, or "password" for
// whoever knows the shared password.
func (u User) ID() string {
	if u.Email != "" {
		return u.Email
	}
	return "password"
}

type Options struct {
	Secret        string
	TTL           time.Duration
	SecureCookies bool
	Password      string  // password mode
	Google        *Google // google mode; nil = password mode
}

type Auth struct {
	mode     Mode
	key      []byte
	password []byte
	ttl      time.Duration
	secure   bool
	limiter  *limiter
	google   *Google
}

func New(o Options) *Auth {
	mode, material := ModePassword, "password\x00"+o.Password
	if o.Google != nil {
		mode, material = ModeGoogle, "google"
		o.Google.init()
	}
	// Deriving the signing key from the mode and password invalidates
	// sessions when either changes.
	mac := hmac.New(sha256.New, []byte(o.Secret))
	mac.Write([]byte(material))
	return &Auth{
		mode:     mode,
		key:      mac.Sum(nil),
		password: []byte(o.Password),
		ttl:      o.TTL,
		secure:   o.SecureCookies,
		limiter:  &limiter{fails: map[string][]time.Time{}},
		google:   o.Google,
	}
}

func (a *Auth) Mode() Mode { return a.mode }

// Login checks the shared password and sets the session cookie.
func (a *Auth) Login(c *gin.Context, password string) (ok bool, retryAfter time.Duration) {
	if a.mode != ModePassword {
		return false, 0
	}
	ip := c.ClientIP()
	if wait := a.limiter.blocked(ip); wait > 0 {
		return false, wait
	}
	if subtle.ConstantTimeCompare([]byte(password), a.password) != 1 {
		a.limiter.fail(ip)
		return false, 0
	}
	a.limiter.reset(ip)
	a.issue(c, claims{RegisteredClaims: jwt.RegisteredClaims{Subject: passwordSubject}})
	return true, 0
}

func (a *Auth) Logout(c *gin.Context) { a.setCookie(c, CookieName, "/", "", -1) }

// claims is the session token.
type claims struct {
	jwt.RegisteredClaims
	Name    string `json:"name,omitempty"`
	Picture string `json:"pic,omitempty"`
}

func (cl *claims) user() User {
	if cl.Subject == passwordSubject {
		return User{}
	}
	return User{Email: cl.Subject, Name: cl.Name, Picture: cl.Picture}
}

func (a *Auth) issue(c *gin.Context, cl claims) {
	now := time.Now()
	cl.IssuedAt = jwt.NewNumericDate(now)
	cl.ExpiresAt = jwt.NewNumericDate(now.Add(a.ttl))
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, cl).SignedString(a.key)
	if err != nil {
		return
	}
	a.setCookie(c, CookieName, "/", signed, int(a.ttl.Seconds()))
}

func (a *Auth) setCookie(c *gin.Context, name, path, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// session returns the claims of a valid session for the current mode.
func (a *Auth) session(raw string) (*claims, bool) {
	cl := &claims{}
	_, err := jwt.ParseWithClaims(raw, cl, func(*jwt.Token) (any, error) { return a.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, false
	}
	if a.mode == ModeGoogle {
		// Someone removed from ALLOWED_EMAILS loses access at once.
		return cl, a.google.allowed(cl.Subject)
	}
	return cl, cl.Subject == passwordSubject
}

// Peek returns who the request's session belongs to, without requiring one.
func (a *Auth) Peek(c *gin.Context) (User, bool) {
	raw, err := c.Cookie(CookieName)
	if err != nil || raw == "" {
		return User{}, false
	}
	cl, ok := a.session(raw)
	if !ok {
		return User{}, false
	}
	return cl.user(), true
}

const userKey = "auth.user"

// UserOf returns who made the request (set by Middleware).
func UserOf(c *gin.Context) User {
	if u, ok := c.Get(userKey); ok {
		return u.(User)
	}
	return User{}
}

// Middleware rejects requests without a valid session and refreshes the cookie
// once half of its lifetime has passed.
func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(CookieName)
		if err != nil || raw == "" {
			abort(c, "Потрібно увійти")
			return
		}
		cl, ok := a.session(raw)
		if !ok {
			a.Logout(c)
			abort(c, "Сесія завершилась, увійдіть знову")
			return
		}
		if cl.IssuedAt != nil && time.Since(cl.IssuedAt.Time) > a.ttl/2 {
			a.issue(c, *cl)
		}
		c.Set(userKey, cl.user())
		c.Next()
	}
}

func abort(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": msg}})
}

// SafeNext keeps a post-login redirect on this site.
func SafeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") || strings.HasPrefix(next, "/api/") {
		return "/"
	}
	return next
}

// limiter slows down password guessing: 8 failures in 10 minutes block an IP for the rest of the window.
type limiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

const (
	limitWindow = 10 * time.Minute
	limitCount  = 8
)

func (l *limiter) prune(ip string, now time.Time) []time.Time {
	kept := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if now.Sub(t) < limitWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, ip)
		return nil
	}
	l.fails[ip] = kept
	return kept
}

func (l *limiter) blocked(ip string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	f := l.prune(ip, now)
	if len(f) < limitCount {
		return 0
	}
	return limitWindow - now.Sub(f[0])
}

func (l *limiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.fails[ip], time.Now())
}

func (l *limiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}
