package melotrackcreator

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

func validatePassword(db *sqlx.DB, password string) error {
	var hash string

	err := db.Get(&hash, `SELECT value FROM Config WHERE key = 'password'`)
	if err != nil {
		return fmt.Errorf("validatePassword: failed to get password hash: %w", err)
	}

	if hash == "no password" {
		return nil
	}

	if err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("GenerateRefreshToken: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func createSession(db *sqlx.DB) (*Session, error) {
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("createSession: failed to generate refresh token: %w", err)
	}

	session := Session{
		RefreshToken: refreshToken,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(180 * 24 * time.Hour),
	}

	query := `INSERT INTO Sessions (refresh_token, created_at, expires_at)
			  VALUES (?, ?, ?)
			  RETURNING id`

	err = db.QueryRow(query, session.RefreshToken, session.CreatedAt, session.ExpiresAt).Scan(&session.ID)
	if err != nil {
		return nil, fmt.Errorf("createSession: failed to insert session: %w", err)
	}

	return &session, nil
}

func GenerateAccessToken(session Session, secret []byte) (string, int, error) {
	tokenLifeTime := 15 * time.Minute

	claims := jwt.MapClaims{
		"exp":        time.Now().Add(tokenLifeTime).Unix(),
		"iat":        time.Now().Unix(),
		"session_id": session.ID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(secret)
	if err != nil {
		return "", 0, fmt.Errorf("GenerateAccessToken: %w", err)
	}

	return signedToken, int(tokenLifeTime.Seconds()), nil
}

func LoginHandler(db *sqlx.DB, secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if err := validatePassword(db, req.Password); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				log.Printf("[WARN] LoginHandler: failed login attempt | client: %s", c.ClientIP())
				c.JSON(http.StatusUnauthorized, gin.H{"error": "bad password"})
				return
			}
			log.Printf("[ERROR] LoginHandler: validatePassword: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		session, err := createSession(db)
		if err != nil {
			log.Printf("[ERROR] LoginHandler: createSession: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		accessToken, accessTokenLifeTime, err := GenerateAccessToken(*session, secret)
		if err != nil {
			log.Printf("[ERROR] LoginHandler: GenerateAccessToken: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		log.Printf("[INFO] LoginHandler: new session created | session: %d | client: %s", session.ID, c.ClientIP())

		c.SetCookie("refresh_token", session.RefreshToken, 180*24*60*60, "/", "", false, true)

		c.JSON(http.StatusCreated, gin.H{
			"access_token": accessToken,
			"expires_in":   accessTokenLifeTime,
		})
	}
}

func LoginStatusHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var hash string

		if err := db.Get(&hash, `SELECT value FROM Config WHERE key = 'password'`); err != nil {
			log.Printf("[ERROR] LoginStatusHandler: failed to get config: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"password_required": hash != "no password"})
	}
}
