package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

type ProjectSummary struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateProjectInput struct {
	Title string `json:"title" binding:"required"`
}

func GetProjects(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userClaims, exists := c.Get("id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims := userClaims.(map[string]interface{})
		userID := uint(claims["id"].(float64))

		var summaries []ProjectSummary
		query := db.Model(&models.Project{})

		query = query.Where("user_id = ?", userID)

		err := query.Select("id", "title", "created_at").Find(&summaries).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, summaries)
	}
}

func GetProjectByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func CreateProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateProjectInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title is essential"})
			return
		}

		userClaims, exists := c.Get("id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims := userClaims.(map[string]interface{})
		userID := uint(claims["id"].(float64))

		project := models.Project{
			Title:  input.Title,
			UserID: userID,
		}

		if err := db.Create(&project).Error; err != nil {
			if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
				c.JSON(http.StatusConflict, gin.H{"error": "title is already taken"})
				return
			}
		}

		if project.Categories == nil {
			project.Categories = make([]models.Category, 0)
		}

		c.JSON(http.StatusCreated, project)
	}
}

func UpdateProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func DeleteProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
