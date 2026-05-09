package melotrackcreator

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func ListSessionsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := `SELECT * FROM Sessions`

		var sessions []Session

		err := db.Select(&sessions, query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, sessions)
	}
}

func DeleteSessionHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idString := c.Param("id")
		sessionID, err := strconv.Atoi(idString)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
			return
		}

		_, err = db.Exec(`DELETE FROM Sessions WHERE id = ?`, sessionID)
		if err != nil {
			log.Printf("[ERROR] LogoutHandler: failed to delete session %d: %v | client: %s", sessionID, err, c.ClientIP())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		log.Printf("[INFO] LogoutHandler: session %d revoked | client: %s", sessionID, c.ClientIP())
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}
