package melotrackcreator

import "time"

type LoginRequest struct {
	Password string `json:"password" binding:"max=72"`
}

type Session struct {
	ID           int       `db:"id"`
	RefreshToken string    `db:"refresh_token"`
	CreatedAt    time.Time `db:"created_at"`
	ExpiresAt    time.Time `db:"expires_at"`
}
