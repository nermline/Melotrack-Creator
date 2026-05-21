package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/media"
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
		projectID := c.Param("pid")

		if !projectExists(c, db, projectID) {
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

		projectIDStr := c.Param("pid")

		if !projectExists(c, db, projectIDStr) {
			return
		}

		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
			return
		}

		var category models.Category

		err = db.Transaction(func(tx *gorm.DB) error {
			var maxPosition int
			tx.Model(&models.Category{}).
				Where("project_id = ?", projectID).
				Select("COALESCE(MAX(position), -1)").
				Scan(&maxPosition)

			category = models.Category{
				ProjectID: uint(projectID),
				Title:     input.Title,
				Position:  maxPosition + 1,
			}

			return tx.Create(&category).Error
		})

		if err != nil {
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

		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !projectExists(c, db, projectID) {
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
		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !projectExists(c, db, projectID) {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			var category models.Category

			if err := tx.Where("id = ? AND project_id = ?", categoryID, projectID).
				First(&category).Error; err != nil {
				return err
			}

			oldPos := category.Position
			pID := category.ProjectID

			media.CleanCategoryMedia(projectID, categoryID)

			if err := tx.Unscoped().Delete(&category).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.Category{}).
				Where("project_id = ? AND position > ?", pID, oldPos).
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

		c.JSON(http.StatusOK, gin.H{"message": "category and all its media deleted successfully"})
	}
}
// projectExists перевіряє, що проєкт існує (без перевірки власника — проєкти спільні).
func projectExists(c *gin.Context, db *gorm.DB, projectID string) bool {
	var count int64
	if err := db.Model(&models.Project{}).Where("id = ?", projectID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return false
	}
	return true
}
