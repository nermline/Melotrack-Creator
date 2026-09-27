// Package auth protects the organiser side with one shared password.
//
// A successful login sets a signed JWT in an HttpOnly cookie. Because it is a
// cookie, <video>, <img> and WebSocket requests are authenticated automatically,
// and the session slides forward while in use, so a presentation screen left
// running for hours keeps working. Changing APP_PASSWORD logs everyone out.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const CookieName = "mt_session"

type Auth struct {
	key      []byte
	password []byte
	ttl      time.Duration
	secure   bool
	limiter  *limiter
}

func New(secret, password string, ttl time.Duration, secureCookies bool) *Auth {
	// Deriving the signing key from the password invalidates sessions when it changes.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(password))
	return &Auth{
		key:      mac.Sum(nil),
		password: []byte(password),
		ttl:      ttl,
		secure:   secureCookies,
		limiter:  &limiter{fails: map[string][]time.Time{}},
	}
}

// Login checks the password and sets the session cookie.
func (a *Auth) Login(c *gin.Context, password string) (ok bool, retryAfter time.Duration) {
	ip := c.ClientIP()
	if wait := a.limiter.blocked(ip); wait > 0 {
		return false, wait
	}
	if subtle.ConstantTimeCompare([]byte(password), a.password) != 1 {
		a.limiter.fail(ip)
		return false, 0
	}
	a.limiter.reset(ip)
	a.issue(c)
	return true, 0
}

func (a *Auth) Logout(c *gin.Context) { a.setCookie(c, "", -1) }

// Valid reports whether the request carries a live session.
func (a *Auth) Valid(c *gin.Context) bool {
	raw, err := c.Cookie(CookieName)
	if err != nil || raw == "" {
		return false
	}
	_, err = a.parse(raw)
	return err == nil
}

func (a *Auth) issue(c *gin.Context) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "organiser",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
	})
	signed, err := tok.SignedString(a.key)
	if err != nil {
		return
	}
	a.setCookie(c, signed, int(a.ttl.Seconds()))
}

func (a *Auth) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) parse(raw string) (*jwt.RegisteredClaims, error) {
	cl := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(raw, cl, func(*jwt.Token) (any, error) { return a.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	return cl, err
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
		cl, err := a.parse(raw)
		if err != nil {
			a.Logout(c)
			abort(c, "Сесія завершилась, увійдіть знову")
			return
		}
		if cl.IssuedAt != nil && time.Since(cl.IssuedAt.Time) > a.ttl/2 {
			a.issue(c)
		}
		c.Next()
	}
}

func abort(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": msg}})
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
