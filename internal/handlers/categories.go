package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

type CreateCategoryInput struct {
	Title string `json:"title" binding:"required"`
}

func GetCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("id")

		if !verifyProjectOwnership(c, db, projectID, userID) {
			return
		}

		var categories []models.Category

		if err := db.Where("project_id = ?", projectID).Order("position ASC").Find(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if categories == nil {
			categories = make([]models.Category, 0)
		} else {
			for i := range categories {
				if categories[i].Items == nil {
					categories[i].Items = make([]models.QuizItem, 0)
				}
			}
		}

		c.JSON(http.StatusOK, categories)
	}
}

func CreateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateCategoryInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
			return
		}

		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectIDStr := c.Param("id")

		if !verifyProjectOwnership(c, db, projectIDStr, userID) {
			return
		}

		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
			return
		}

		var maxPosition int
		db.Model(&models.Category{}).
			Where("project_id = ?", projectID).
			Select("COALESCE(MAX(position), -1)").
			Scan(&maxPosition)

		category := models.Category{
			ProjectID: uint(projectID),
			Title:     input.Title,
			Position:  maxPosition + 1,
		}

		if err := db.Create(&category).Error; err != nil {
			if isUniqueErr(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "category title must be unique within the project"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if category.Items == nil {
			category.Items = make([]models.QuizItem, 0)
		}

		c.JSON(http.StatusCreated, category)
	}
}

func UpdateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func DeleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func verifyProjectOwnership(c *gin.Context, db *gorm.DB, projectID string, userID uint) bool {
	var project models.Project
	if err := db.Select("id").Where("id = ? AND user_id = ?", projectID, userID).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return false
	}
	return true
}
