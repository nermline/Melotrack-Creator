package melotrackcreator

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func rotateRefreshToken(sessionID int, db *sqlx.DB) (*Session, error) {
	var session Session

	err := db.Get(&session, `SELECT * FROM Sessions WHERE id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("rotateRefreshToken: failed to get session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("rotateRefreshToken: session %d expired at %v", session.ID, session.ExpiresAt)
	}

	newToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("rotateRefreshToken: failed to generate new token: %w", err)
	}

	session.RefreshToken = newToken
	session.CreatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(180 * 24 * time.Hour)

	_, err = db.Exec(
		`UPDATE Sessions SET refresh_token = ?, created_at = ?, expires_at = ? WHERE id = ?`,
		session.RefreshToken,
		session.CreatedAt,
		session.ExpiresAt,
		session.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("rotateRefreshToken: failed to update session: %w", err)
	}

	return &session, nil
}

func getSessionID(db *sqlx.DB, refreshToken string) (int, error) {
	var sessionID int

	err := db.Get(&sessionID, `SELECT id FROM Sessions WHERE refresh_token = ?`, refreshToken)
	if err != nil {
		return 0, err
	}

	return sessionID, nil
}

func RefreshHandler(db *sqlx.DB, secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		refreshToken, err := c.Cookie("refresh_token")
		if err != nil {
			log.Printf("[WARN] RefreshHandler: no cookie | client: %s", c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no refresh token cookie"})
			return
		}

		sessionID, err := getSessionID(db, refreshToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Printf("[WARN] RefreshHandler: refresh token not found | client: %s", c.ClientIP())
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			} else {
				log.Printf("[ERROR] RefreshHandler: getSessionID: %v | client: %s", err, c.ClientIP())
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}

		session, err := rotateRefreshToken(sessionID, db)
		if err != nil {
			log.Printf("[ERROR] RefreshHandler: rotateRefreshToken: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			return
		}

		accessToken, expiresIn, err := GenerateAccessToken(*session, secret)
		if err != nil {
			log.Printf("[ERROR] RefreshHandler: GenerateAccessToken: %v | client: %s", err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		log.Printf("[INFO] RefreshHandler: session %d rotated | client: %s", session.ID, c.ClientIP())

		c.SetCookie("refresh_token", session.RefreshToken, 180*24*60*60, "/", "", false, true)

		c.JSON(http.StatusOK, gin.H{
			"access_token": accessToken,
			"expires_in":   expiresIn,
		})
	}
}
