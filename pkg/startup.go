package melotrackcreator

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func InitDB() (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite", "./data.db")
	if err != nil {
		return nil, fmt.Errorf("InitDB: failed to initialize database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS Config (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	
	CREATE TABLE IF NOT EXISTS Sessions (
		id INTEGER PRIMARY KEY,
		refresh_token TEXT,
		created_at DATETIME,
		expires_at DATETIME
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("InitDB: failed to apply schema: %w", err)
	}

	return db, nil
}

func SetupPassword(db *sqlx.DB) error {
	var hash string

	err := db.Get(&hash, "SELECT value FROM Config WHERE key = 'password'")
	if err == nil {
		return nil
	}

	if err != sql.ErrNoRows {
		return fmt.Errorf("SetupPassword: failed to check existing password: %w", err)
	}

	fmt.Println("Before starting you must configure password! You can skip by entering nothing.")

	for {
		fmt.Print("\nNew password: ")
		bytePassword1, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("SetupPassword: failed to read password: %w", err)
		}
		if len(bytePassword1) > 72 {
			fmt.Println("\nPassword is too long. Try again!")
			continue
		}

		fmt.Print("\nConfirm password: ")
		bytePassword2, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("SetupPassword: failed to read password confirmation: %w", err)
		}
		fmt.Println()

		if bytes.Equal(bytePassword1, bytePassword2) {
			var finalHash string

			if len(bytePassword1) == 0 {
				finalHash = "no password"
			} else {
				hashBytes, err := bcrypt.GenerateFromPassword(bytePassword1, bcrypt.DefaultCost)
				if err != nil {
					return fmt.Errorf("SetupPassword: failed to generate password hash: %w", err)
				}
				finalHash = string(hashBytes)
			}

			_, err = db.Exec("INSERT INTO Config (key, value) VALUES ('password', ?)", finalHash)
			if err != nil {
				return fmt.Errorf("SetupPassword: failed to save password: %w", err)
			}
			break
		} else {
			fmt.Println("Passwords don't match. Try again!")
		}
	}

	fmt.Println("\nPassword saved successfully!")
	return nil
}

func GetJWTSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("GetJWTSecret: environment variable JWT_SECRET is not set")
	}
	return []byte(secret), nil
}
