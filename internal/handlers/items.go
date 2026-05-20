package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

var activeWorkers sync.Map

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

type UpdateVideoInput struct {
	YouTubeURL *string  `json:"youtube_url"`
	StartTime  *float64 `json:"start_time"`
	EndTime    *float64 `json:"end_time"`
	Volume     *float64 `json:"volume"`
	CropX      *int     `json:"crop_x"`
	CropY      *int     `json:"crop_y"`
	CropWidth  *int     `json:"crop_width"`
	CropHeight *int     `json:"crop_height"`
}

type AnswerInput struct {
	Title           string `json:"title"`
	ImageCropX      int    `json:"image_crop_x"`
	ImageCropY      int    `json:"image_crop_y"`
	ImageCropWidth  int    `json:"image_crop_width"`
	ImageCropHeight int    `json:"image_crop_height"`
}

type UpdateAnswerInput struct {
	Title           *string `json:"title"`
	ImageCropX      *int    `json:"image_crop_x"`
	ImageCropY      *int    `json:"image_crop_y"`
	ImageCropWidth  *int    `json:"image_crop_width"`
	ImageCropHeight *int    `json:"image_crop_height"`
}

type CreateQuizItemInput struct {
	ShowVideo bool        `json:"show_video"`
	Video     VideoInput  `json:"video" binding:"required"`
	Answer    AnswerInput `json:"answer" binding:"required"`
}

type UpdateQuizItemInput struct {
	ShowVideo *bool              `json:"show_video"`
	Position  *int               `json:"position"`
	Video     *UpdateVideoInput  `json:"video"`
	Answer    *UpdateAnswerInput `json:"answer"`
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
				YouTubeURL:       input.Video.YouTubeURL,
				StartTime:        input.Video.StartTime,
				EndTime:          input.Video.EndTime,
				Volume:           input.Video.Volume,
				CropX:            input.Video.CropX,
				CropY:            input.Video.CropY,
				CropWidth:        input.Video.CropWidth,
				CropHeight:       input.Video.CropHeight,
				ProcessingStatus: "pending",
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

		ctx, cancel := context.WithCancel(context.Background())

		activeWorkers.Store(item.ID, cancel)

		go func(itemID uint, pID string) {
			defer activeWorkers.Delete(itemID)

			media.StartVideoProcessingWorker(ctx, db, itemID, true, pID)
		}(item.ID, projectID)

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
		var urlChanged bool
		var paramsChanged bool

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Preload("Answer").Where("id = ? AND category_id = ?", itemID, categoryID).First(&updatedItem).Error; err != nil {
				return err
			}

			if input.ShowVideo != nil {
				updatedItem.ShowVideo = *input.ShowVideo
			}

			if input.Video != nil {
				if input.Video.YouTubeURL != nil && updatedItem.Video.YouTubeURL != *input.Video.YouTubeURL {
					urlChanged = true
					updatedItem.Video.YouTubeURL = *input.Video.YouTubeURL
				}

				if input.Video.StartTime != nil && updatedItem.Video.StartTime != *input.Video.StartTime {
					paramsChanged = true
					updatedItem.Video.StartTime = *input.Video.StartTime
				}
				if input.Video.EndTime != nil && updatedItem.Video.EndTime != *input.Video.EndTime {
					paramsChanged = true
					updatedItem.Video.EndTime = *input.Video.EndTime
				}
				if input.Video.Volume != nil && updatedItem.Video.Volume != *input.Video.Volume {
					paramsChanged = true
					updatedItem.Video.Volume = *input.Video.Volume
				}
				if input.Video.CropX != nil && updatedItem.Video.CropX != *input.Video.CropX {
					paramsChanged = true
					updatedItem.Video.CropX = *input.Video.CropX
				}
				if input.Video.CropY != nil && updatedItem.Video.CropY != *input.Video.CropY {
					paramsChanged = true
					updatedItem.Video.CropY = *input.Video.CropY
				}
				if input.Video.CropWidth != nil && updatedItem.Video.CropWidth != *input.Video.CropWidth {
					paramsChanged = true
					updatedItem.Video.CropWidth = *input.Video.CropWidth
				}
				if input.Video.CropHeight != nil && updatedItem.Video.CropHeight != *input.Video.CropHeight {
					paramsChanged = true
					updatedItem.Video.CropHeight = *input.Video.CropHeight
				}
			}

			if input.Answer != nil {
				if input.Answer.Title != nil {
					updatedItem.Answer.Title = *input.Answer.Title
				}
				if input.Answer.ImageCropX != nil {
					updatedItem.Answer.ImageCropX = *input.Answer.ImageCropX
				}
				if input.Answer.ImageCropY != nil {
					updatedItem.Answer.ImageCropY = *input.Answer.ImageCropY
				}
				if input.Answer.ImageCropWidth != nil {
					updatedItem.Answer.ImageCropWidth = *input.Answer.ImageCropWidth
				}
				if input.Answer.ImageCropHeight != nil {
					updatedItem.Answer.ImageCropHeight = *input.Answer.ImageCropHeight
				}

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

			if urlChanged || paramsChanged {
				updatedItem.Video.ProcessingStatus = "pending"
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

		if urlChanged || paramsChanged {
			if cancelFunc, exists := activeWorkers.Load(updatedItem.ID); exists {
				cancelFunc.(context.CancelFunc)()
			}

			ctx, cancel := context.WithCancel(context.Background())

			activeWorkers.Store(updatedItem.ID, cancel)

			go func(itemID uint, uChanged bool, pID string) {
				defer activeWorkers.Delete(itemID)

				media.StartVideoProcessingWorker(ctx, db, itemID, uChanged, pID)
			}(updatedItem.ID, urlChanged, projectID)
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
