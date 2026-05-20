package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

type CreateCategoryInput struct {
	Title string `json:"title" binding:"required"`
}

type UpdateCategoryInput struct {
	Title    *string `json:"title"`
	Position *int    `json:"position"`
}

func GetCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")

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

		projectIDStr := c.Param("pid")

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
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			var maxPosition int
			tx.Model(&models.Category{}).
				Where("project_id = ?", projectID).
				Select("COALESCE(MAX(position), -1)").
				Scan(&maxPosition)

			category.Position = maxPosition + 1

			return tx.Create(&category).Error
		})

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
		var input UpdateCategoryInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input data"})
			return
		}

		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !verifyProjectOwnership(c, db, projectID, userID) {
			return
		}

		var updatedCategory models.Category
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("id = ? AND project_id = ?", categoryID, projectID).
				First(&updatedCategory).Error; err != nil {
				return err
			}

			if input.Title != nil {
				updatedCategory.Title = *input.Title
			}

			if input.Position != nil {
				newPos := *input.Position
				oldPos := updatedCategory.Position

				if newPos != oldPos {
					var count int64
					tx.Model(&models.Category{}).Where("project_id = ?", updatedCategory.ProjectID).Count(&count)
					maxPos := int(count) - 1

					if newPos > maxPos {
						newPos = maxPos
					}
					if newPos < 0 {
						newPos = 0
					}

					if newPos > oldPos {
						tx.Model(&models.Category{}).
							Where("project_id = ? AND position > ? AND position <= ?", updatedCategory.ProjectID, oldPos, newPos).
							UpdateColumn("position", gorm.Expr("position - 1"))
					} else if newPos < oldPos {
						tx.Model(&models.Category{}).
							Where("project_id = ? AND position >= ? AND position < ?", updatedCategory.ProjectID, newPos, oldPos).
							UpdateColumn("position", gorm.Expr("position + 1"))
					}

					updatedCategory.Position = newPos
				}
			}

			if err := tx.Save(&updatedCategory).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
				return
			}
			if isUniqueErr(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "category title must be unique within the project"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, updatedCategory)
	}
}

func DeleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}
		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !verifyProjectOwnership(c, db, projectID, userID) {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			var category models.Category

			if err := tx.Where("id = ? AND project_id = ?", categoryID, projectID).
				First(&category).Error; err != nil {
				return err
			}

			oldPos := category.Position
			projectID := category.ProjectID

			if err := tx.Unscoped().Delete(&category).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.Category{}).
				Where("project_id = ? AND position > ?", projectID, oldPos).
				UpdateColumn("position", gorm.Expr("position - 1")).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete category"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "category deleted successfully"})
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
