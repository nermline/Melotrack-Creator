package melotrackcreator

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func LogoutHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := GetIDFromContext(c, "sessionID")
		if err != nil {
			log.Printf("[ERROR] LogoutHandler: GetIDFromContext: %v | client: %s", err, c.ClientIP())
			c.SetCookie("refresh_token", "", -1, "/", "", false, true)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		_, err = db.Exec(`DELETE FROM Sessions WHERE id = ?`, sessionID)
		if err != nil {
			log.Printf("[ERROR] LogoutHandler: failed to delete session %d: %v | client: %s", sessionID, err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		log.Printf("[INFO] LogoutHandler: session %d revoked | client: %s", sessionID, c.ClientIP())

		c.SetCookie("refresh_token", "", -1, "/", "", false, true)

		c.JSON(http.StatusOK, gin.H{"message": "logout"})
	}
}
