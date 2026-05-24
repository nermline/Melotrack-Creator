package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/media"
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
		var summaries []ProjectSummary

		if err := db.Model(&models.Project{}).Select("id", "title", "created_at").Order("created_at DESC").Find(&summaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, summaries)
	}
}

func GetProjectByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")

		var project models.Project

		err := db.
			Preload("Categories").
			Preload("Categories.Items").
			Preload("Categories.Items.Answer").
			Preload("Categories.Items.Media").
			Where("id = ?", projectID).
			First(&project).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
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

		project := models.Project{
			Title: input.Title,
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

		projectID := c.Param("pid")

		var project models.Project

		if err := db.Where("id = ?", projectID).First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
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
		projectID := c.Param("pid")

		media.CleanProjectMedia(projectID)

		result := db.Unscoped().Where("id = ?", projectID).Delete(&models.Project{})

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "project and all its associated media deleted successfully"})
	}
}

func isUniqueErr(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate")
}
