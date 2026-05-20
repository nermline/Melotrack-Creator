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

type ProjectInput struct {
	Title string `json:"title" binding:"required"`
}

func GetProjects(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		var summaries []ProjectSummary

		if err := db.Model(&models.Project{}).Where("user_id = ?", userID).Select("id", "title", "created_at").Find(&summaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, summaries)
	}
}

func GetProjectByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")

		var project models.Project

		err := db.
			Preload("Categories").
			Preload("Categories.Items").
			Preload("Categories.Items.Answer").
			Preload("Categories.Items.Media").
			Where("id = ? AND user_id = ?", projectID, userID).
			First(&project).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if project.Categories == nil {
			project.Categories = make([]models.Category, 0)
		} else {
			for i := range project.Categories {
				if project.Categories[i].Items == nil {
					project.Categories[i].Items = make([]models.QuizItem, 0)
				}
			}
		}

		c.JSON(http.StatusOK, project)
	}
}

func CreateProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ProjectInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title is essential"})
			return
		}

		userID, ok := getUserID(c)
		if !ok {
			return
		}

		project := models.Project{
			Title:  input.Title,
			UserID: userID,
		}

		if err := db.Create(&project).Error; err != nil {
			if isUniqueErr(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "title is already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if project.Categories == nil {
			project.Categories = make([]models.Category, 0)
		}

		c.JSON(http.StatusCreated, project)
	}
}

func UpdateProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ProjectInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title is essential"})
			return
		}

		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")

		var project models.Project

		if err := db.Where("id = ? AND user_id = ?", projectID, userID).First(&project).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		project.Title = input.Title

		if err := db.Save(&project).Error; err != nil {
			if isUniqueErr(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "title is already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update project"})
			return
		}

		if project.Categories == nil {
			project.Categories = make([]models.Category, 0)
		}

		c.JSON(http.StatusOK, project)
	}
}

func DeleteProject(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")

		result := db.Unscoped().Where("id = ? AND user_id = ?", projectID, userID).Delete(&models.Project{})

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "project deleted successfully"})
	}
}

// getUserID extracts the authenticated user's ID from the JWT claims stored in the context.
// Returns (0, false) and writes an error response if the claim is missing or has an unexpected type.
func getUserID(c *gin.Context) (uint, bool) {
	userClaims, exists := c.Get("id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	claims, ok := userClaims.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	idFloat, ok := claims["id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return 0, false
	}

	return uint(idFloat), true
}

func isUniqueErr(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate")
}
