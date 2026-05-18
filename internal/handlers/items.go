package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

type VideoInput struct {
	YouTubeURL string  `json:"youtube_url"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Volume     float64 `json:"volume"`
	CropX      int     `json:"crop_x"`
	CropY      int     `json:"crop_y"`
	CropWidth  int     `json:"crop_width"`
	CropHeight int     `json:"crop_height"`
}

type AnswerInput struct {
	Title           string `json:"title"`
	ImageCropX      int    `json:"image_crop_x"`
	ImageCropY      int    `json:"image_crop_y"`
	ImageCropWidth  int    `json:"image_crop_width"`
	ImageCropHeight int    `json:"image_crop_height"`
}

type CreateQuizItemInput struct {
	ShowVideo bool        `json:"show_video"`
	Video     VideoInput  `json:"video" binding:"required"`
	Answer    AnswerInput `json:"answer" binding:"required"`
}

type UpdateQuizItemInput struct {
	ShowVideo bool         `json:"show_video"`
	Position  *int         `json:"position"`
	Video     *VideoInput  `json:"video"`
	Answer    *AnswerInput `json:"answer"`
}

func verifyCategoryOwnership(c *gin.Context, db *gorm.DB, projectID string, categoryID string, userID uint) bool {
	if !verifyProjectOwnership(c, db, projectID, userID) {
		return false
	}
	var count int64
	db.Model(&models.Category{}).Where("id = ? AND project_id = ?", categoryID, projectID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found in this project"})
		return false
	}
	return true
}

func GetQuizItems(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !verifyCategoryOwnership(c, db, projectID, categoryID, userID) {
			return
		}

		var items []models.QuizItem
		if err := db.Preload("Answer").Where("category_id = ?", categoryID).Order("position ASC").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if items == nil {
			items = make([]models.QuizItem, 0)
		}

		c.JSON(http.StatusOK, items)
	}
}

func CreateQuizItem(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateQuizItemInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input data"})
			return
		}

		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")
		categoryIDStr := c.Param("cid")

		if !verifyCategoryOwnership(c, db, projectID, categoryIDStr, userID) {
			return
		}

		categoryID, _ := strconv.ParseUint(categoryIDStr, 10, 32)

		var maxPosition int
		db.Model(&models.QuizItem{}).
			Where("category_id = ?", categoryID).
			Select("COALESCE(MAX(position), -1)").
			Scan(&maxPosition)

		item := models.QuizItem{
			CategoryID: uint(categoryID),
			Position:   maxPosition + 1,
			Video: models.Video{
				YouTubeURL: input.Video.YouTubeURL,
				StartTime:  input.Video.StartTime,
				EndTime:    input.Video.EndTime,
				Volume:     input.Video.Volume,
				CropX:      input.Video.CropX,
				CropY:      input.Video.CropY,
				CropWidth:  input.Video.CropWidth,
				CropHeight: input.Video.CropHeight,
			},
			Answer: models.Answer{
				Title:           input.Answer.Title,
				ImageCropX:      input.Answer.ImageCropX,
				ImageCropY:      input.Answer.ImageCropY,
				ImageCropWidth:  input.Answer.ImageCropWidth,
				ImageCropHeight: input.Answer.ImageCropHeight,
			},
			ShowVideo: input.ShowVideo,
		}

		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create item"})
			return
		}

		c.JSON(http.StatusCreated, item)
	}
}

func UpdateQuizItem(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input UpdateQuizItemInput
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
		itemID := c.Param("iid")

		if !verifyCategoryOwnership(c, db, projectID, categoryID, userID) {
			return
		}

		var updatedItem models.QuizItem
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Preload("Answer").Where("id = ? AND category_id = ?", itemID, categoryID).First(&updatedItem).Error; err != nil {
				return err
			}

			if input.Video != nil {
				updatedItem.Video.YouTubeURL = input.Video.YouTubeURL
				updatedItem.Video.StartTime = input.Video.StartTime
				updatedItem.Video.EndTime = input.Video.EndTime
				updatedItem.Video.Volume = input.Video.Volume
				updatedItem.Video.CropX = input.Video.CropX
				updatedItem.Video.CropY = input.Video.CropY
				updatedItem.Video.CropWidth = input.Video.CropWidth
				updatedItem.Video.CropHeight = input.Video.CropHeight
			}

			if input.Answer != nil {
				updatedItem.Answer.Title = input.Answer.Title
				updatedItem.Answer.ImageCropX = input.Answer.ImageCropX
				updatedItem.Answer.ImageCropY = input.Answer.ImageCropY
				updatedItem.Answer.ImageCropWidth = input.Answer.ImageCropWidth
				updatedItem.Answer.ImageCropHeight = input.Answer.ImageCropHeight
				if err := tx.Save(&updatedItem.Answer).Error; err != nil {
					return err
				}
			}

			if input.Position != nil {
				newPos := *input.Position
				oldPos := updatedItem.Position

				if newPos != oldPos {
					var count int64
					tx.Model(&models.QuizItem{}).Where("category_id = ?", updatedItem.CategoryID).Count(&count)
					maxPos := int(count) - 1

					if newPos > maxPos {
						newPos = maxPos
					}
					if newPos < 0 {
						newPos = 0
					}

					if newPos > oldPos {
						tx.Model(&models.QuizItem{}).
							Where("category_id = ? AND position > ? AND position <= ?", updatedItem.CategoryID, oldPos, newPos).
							UpdateColumn("position", gorm.Expr("position - 1"))
					} else if newPos < oldPos {
						tx.Model(&models.QuizItem{}).
							Where("category_id = ? AND position >= ? AND position < ?", updatedItem.CategoryID, newPos, oldPos).
							UpdateColumn("position", gorm.Expr("position + 1"))
					}
					updatedItem.Position = newPos
				}
			}

			return tx.Save(&updatedItem).Error
		})

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, updatedItem)
	}
}

func DeleteQuizItem(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserID(c)
		if !ok {
			return
		}

		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !verifyCategoryOwnership(c, db, projectID, categoryID, userID) {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			var item models.QuizItem
			if err := tx.Where("id = ? AND category_id = ?", itemID, categoryID).First(&item).Error; err != nil {
				return err
			}

			oldPos := item.Position
			catID := item.CategoryID

			if err := tx.Unscoped().Delete(&item).Error; err != nil {
				return err
			}

			return tx.Model(&models.QuizItem{}).
				Where("category_id = ? AND position > ?", catID, oldPos).
				UpdateColumn("position", gorm.Expr("position - 1")).Error
		})

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "item deleted successfully"})
	}
}
