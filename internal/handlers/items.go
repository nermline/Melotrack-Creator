package handlers

import (
	"context"
	"errors"
	"fmt"
	_ "image/png"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"github.com/nermline/Melotrack-Creator/ws"
	"gorm.io/gorm"
)

var (
	activeDownloadWorkers sync.Map // mediaID -> cancelFunc
	activeRenderWorkers   sync.Map // itemID -> cancelFunc
)

// triggerDownload запускає фонове завантаження оригіналу для media у горутині.
// Винесено у змінну, щоб тести могли підмінити її без реального виклику yt-dlp.
// Якщо для цього media вже активне завантаження — повторно не запускає.
var triggerDownload = func(db *gorm.DB, mediaID uint, url string) {
	if _, active := activeDownloadWorkers.Load(mediaID); active {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	activeDownloadWorkers.Store(mediaID, cancel)
	go func() {
		defer activeDownloadWorkers.Delete(mediaID)
		media.StartDownloadWorker(ctx, db, mediaID, url)
	}()
}

// triggerRender запускає фоновий рендер (ffmpeg) у горутині. Винесено у змінну,
// щоб тести могли підмінити її без реального виклику ffmpeg.
var triggerRender = func(db *gorm.DB, hub *ws.Hub, item models.QuizItem, projectID, categoryID string) {
	ctx, cancel := context.WithCancel(context.Background())
	activeRenderWorkers.Store(item.ID, cancel)
	go func() {
		defer activeRenderWorkers.Delete(item.ID)
		media.StartRenderWorker(ctx, db, hub, item, projectID, categoryID)
	}()
}

type VideoInput struct {
	YoutubeURL string  `json:"youtube_url"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Volume     float64 `json:"volume"`
	CropX      int     `json:"crop_x"`
	CropY      int     `json:"crop_y"`
	CropWidth  int     `json:"crop_width"`
	CropHeight int     `json:"crop_height"`
	Fit        bool    `json:"fit"`
}

type UpdateVideoInput struct {
	YoutubeURL *string  `json:"youtube_url"`
	StartTime  *float64 `json:"start_time"`
	EndTime    *float64 `json:"end_time"`
	Volume     *float64 `json:"volume"`
	CropX      *int     `json:"crop_x"`
	CropY      *int     `json:"crop_y"`
	CropWidth  *int     `json:"crop_width"`
	CropHeight *int     `json:"crop_height"`
	Fit        *bool    `json:"fit"`
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

// categoryExists перевіряє, що категорія існує в межах проєкту (проєкти спільні).
func categoryExists(c *gin.Context, db *gorm.DB, projectID string, categoryID string) bool {
	if !projectExists(c, db, projectID) {
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
		projectID := c.Param("pid")
		categoryID := c.Param("cid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		var items []models.QuizItem
		if err := db.Preload("Answer").Preload("Video.Media").Where("category_id = ?", categoryID).Order("position ASC").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if items == nil {
			items = make([]models.QuizItem, 0)
		}

		c.JSON(http.StatusOK, items)
	}
}

func CreateQuizItem(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CreateQuizItemInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input data"})
			return
		}

		projectID := c.Param("pid")
		categoryIDStr := c.Param("cid")

		if !categoryExists(c, db, projectID, categoryIDStr) {
			return
		}

		categoryID, _ := strconv.ParseUint(categoryIDStr, 10, 32)
		ytID := media.ExtractYouTubeID(input.Video.YoutubeURL)
		if ytID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid youtube url"})
			return
		}
		// Зберігаємо очищений URL (без &list= та інших параметрів плейлисту)
		cleanYtURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", ytID)

		var item models.QuizItem
		var mediaFile models.Media
		var needDownload bool

		err := db.Transaction(func(tx *gorm.DB) error {
			// 1. Отримуємо або створюємо Media
			err := tx.Where("you_tube_id = ?", ytID).First(&mediaFile).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				mediaFile = models.Media{
					YouTubeID: ytID,
					Status:    "downloading",
				}
				if err := tx.Create(&mediaFile).Error; err != nil {
					return err
				}
				needDownload = true
			} else if err != nil {
				return err
			} else if mediaFile.Status == "error" {
				// Попередня спроба завантаження провалилась — перезапускаємо.
				if err := tx.Model(&mediaFile).Update("status", "downloading").Error; err != nil {
					return err
				}
				mediaFile.Status = "downloading"
				needDownload = true
			}

			// 2. Створюємо QuizItem
			var maxPosition int
			tx.Model(&models.QuizItem{}).
				Where("category_id = ?", categoryID).
				Select("COALESCE(MAX(position), -1)").
				Scan(&maxPosition)

			item = models.QuizItem{
				CategoryID: uint(categoryID),
				Position:   maxPosition + 1,
				Video: models.Video{
					YouTubeURL:   cleanYtURL,
					MediaID:      &mediaFile.ID,
					StartTime:    input.Video.StartTime,
					EndTime:      input.Video.EndTime,
					Volume:       input.Video.Volume,
					CropX:        input.Video.CropX,
					CropY:        input.Video.CropY,
					CropWidth:    input.Video.CropWidth,
					CropHeight:   input.Video.CropHeight,
					Fit:          input.Video.Fit,
					RenderStatus: "unrendered",
				},
				Answer: models.Answer{
					Title: input.Answer.Title,
				},
				ShowVideo: input.ShowVideo,
			}

			return tx.Create(&item).Error
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create item"})
			return
		}

		// Запускаємо завантаження оригінального відео, якщо його ще немає в базі
		// (або попередня спроба провалилась).
		if needDownload {
			triggerDownload(db, mediaFile.ID, item.Video.YouTubeURL)
		}
		item.Video.Media = mediaFile

		hub.SystemBroadcast(categoryIDStr, ws.EditorMessage{
			Action: "item_created",
			ItemID: item.ID,
			Data:   item,
		})

		c.JSON(http.StatusCreated, item)
	}
}

// validateVideoCropParams перевіряє коректність часових меж та crop-прямокутника
// відносно реальних розмірів і тривалості завантаженого відео.
// Якщо метадані ще не заповнені (width/height/duration == 0) — пропускаємо перевірку.
func validateVideoCropParams(v models.Video, m models.Media) error {
	if m.Width == 0 || m.Height == 0 || m.Duration == 0 {
		return nil // метадані ще не завантажені, перевірка пізніше
	}

	// Часові мітки
	if v.StartTime < 0 {
		return fmt.Errorf("start_time не може бути від'ємним")
	}
	if v.EndTime > 0 && v.EndTime <= v.StartTime {
		return fmt.Errorf("end_time (%.2f) має бути більшим за start_time (%.2f)", v.EndTime, v.StartTime)
	}
	if v.StartTime >= m.Duration {
		return fmt.Errorf("start_time (%.2fs) виходить за межі тривалості відео (%.2fs)", v.StartTime, m.Duration)
	}
	if v.EndTime > 0 && v.EndTime > m.Duration {
		return fmt.Errorf("end_time (%.2fs) виходить за межі тривалості відео (%.2fs)", v.EndTime, m.Duration)
	}

	// У режимі "вмістити" crop ігнорується — пропускаємо перевірку рамки.
	if v.Fit {
		return nil
	}

	// Crop-прямокутник (перевіряємо лише якщо задано)
	if v.CropWidth > 0 || v.CropHeight > 0 {
		if v.CropX < 0 {
			return fmt.Errorf("crop_x не може бути від'ємним")
		}
		if v.CropY < 0 {
			return fmt.Errorf("crop_y не може бути від'ємним")
		}
		if v.CropWidth <= 0 {
			return fmt.Errorf("crop_width має бути більшим за 0")
		}
		if v.CropHeight <= 0 {
			return fmt.Errorf("crop_height має бути більшим за 0")
		}
		if v.CropX+v.CropWidth > m.Width {
			return fmt.Errorf("crop виходить за правий край: x(%d)+w(%d)=%d > ширина відео(%d)", v.CropX, v.CropWidth, v.CropX+v.CropWidth, m.Width)
		}
		if v.CropY+v.CropHeight > m.Height {
			return fmt.Errorf("crop виходить за нижній край: y(%d)+h(%d)=%d > висота відео(%d)", v.CropY, v.CropHeight, v.CropY+v.CropHeight, m.Height)
		}
	}

	return nil
}

func UpdateQuizItem(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input UpdateQuizItemInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input data"})
			return
		}

		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		var updatedItem models.QuizItem
		var urlChanged bool
		var paramsChanged bool
		var newMediaFile models.Media
		var needDownload bool
		var validationErr error // відокремлюємо помилки валідації від DB-помилок

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Preload("Answer").Preload("Video.Media").Where("id = ? AND category_id = ?", itemID, categoryID).First(&updatedItem).Error; err != nil {
				return err
			}

			if input.ShowVideo != nil {
				updatedItem.ShowVideo = *input.ShowVideo
			}

			if input.Video != nil {
				if input.Video.YoutubeURL != nil && updatedItem.Video.YouTubeURL != *input.Video.YoutubeURL {
					urlChanged = true
					ytID := media.ExtractYouTubeID(*input.Video.YoutubeURL)
					// Зберігаємо очищений URL (без &list= тощо)
					updatedItem.Video.YouTubeURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", ytID)

					err := tx.Where("you_tube_id = ?", ytID).First(&newMediaFile).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						newMediaFile = models.Media{
							YouTubeID: ytID,
							Status:    "downloading",
						}
						if err := tx.Create(&newMediaFile).Error; err != nil {
							return err
						}
						needDownload = true
					} else if err != nil {
						return err
					} else if newMediaFile.Status == "error" {
						// Попередня спроба провалилась — перезавантажуємо.
						if err := tx.Model(&newMediaFile).Update("status", "downloading").Error; err != nil {
							return err
						}
						newMediaFile.Status = "downloading"
						needDownload = true
					}
					updatedItem.Video.MediaID = &newMediaFile.ID
					updatedItem.Video.Media = newMediaFile
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
				if input.Video.Fit != nil && updatedItem.Video.Fit != *input.Video.Fit {
					paramsChanged = true
					updatedItem.Video.Fit = *input.Video.Fit
				}
			}

			if input.Answer != nil {
				// ... (оновлення полів Answer без змін) ...
				if input.Answer.Title != nil {
					updatedItem.Answer.Title = *input.Answer.Title
				}
				if err := tx.Save(&updatedItem.Answer).Error; err != nil {
					return err
				}
			}

			if input.Position != nil {
				// ... (оновлення Position без змін) ...
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
				updatedItem.Video.RenderStatus = "unrendered"
			}

			// Валідація crop та часових меж відносно реальних розмірів відео
			if ve := validateVideoCropParams(updatedItem.Video, updatedItem.Video.Media); ve != nil {
				validationErr = ve
				return ve
			}

			return tx.Save(&updatedItem).Error
		})

		if validationErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
			return
		}
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if needDownload {
			triggerDownload(db, newMediaFile.ID, updatedItem.Video.YouTubeURL)
		}

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_updated",
			ItemID: updatedItem.ID,
			Data:   updatedItem, // Відправляємо повністю оновлений об'єкт
		})

		c.JSON(http.StatusOK, updatedItem)
	}
}

// RetryDownload перезапускає завантаження оригінального відео для item, чий media
// у статусі "error" (або застряг). Дозволяє кнопці "спробувати ще раз" на фронті
// перезавантажити відео без повторного створення елемента.
func RetryDownload(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		var item models.QuizItem
		if err := db.Preload("Video.Media").Where("id = ? AND category_id = ?", itemID, categoryID).First(&item).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}

		if item.Video.MediaID == nil || item.Video.Media.YouTubeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "item has no associated media"})
			return
		}

		// Якщо вже завантажується — не дублюємо.
		if _, active := activeDownloadWorkers.Load(*item.Video.MediaID); active {
			c.JSON(http.StatusConflict, gin.H{"error": "download already in progress"})
			return
		}

		mID := *item.Video.MediaID
		if err := db.Model(&models.Media{}).Where("id = ?", mID).Update("status", "downloading").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		triggerDownload(db, mID, item.Video.YouTubeURL)

		// Сповіщаємо редакторів, щоб оновили статус і почали опитування.
		item.Video.Media.Status = "downloading"
		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_updated",
			ItemID: item.ID,
			Data:   item,
		})

		c.JSON(http.StatusOK, gin.H{"status": "downloading"})
	}
}

// RenderQuizItem - новий ендпоінт для застосування налаштувань (ffmpeg)
func RenderQuizItem(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		var item models.QuizItem
		if err := db.Preload("Video.Media").Where("id = ? AND category_id = ?", itemID, categoryID).First(&item).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}

		if item.Video.Media.Status != "ready" {
			c.JSON(http.StatusConflict, gin.H{"error": "Original video is still downloading or encountered an error"})
			return
		}

		// Валідація crop та часових меж відносно реальних розмірів відео
		if err := validateVideoCropParams(item.Video, item.Video.Media); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Відміняємо попередній рендер, якщо він ще йде
		if cancelFunc, exists := activeRenderWorkers.Load(item.ID); exists {
			cancelFunc.(context.CancelFunc)()
		}

		db.Model(&item).Update("render_status", "rendering")

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_rendering",
			ItemID: item.ID,
			Data:   map[string]string{"render_status": "rendering"},
		})

		triggerRender(db, hub, item, projectID, categoryID)

		c.JSON(http.StatusOK, gin.H{"render_status": "rendering"})
	}
}

func DeleteQuizItem(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			var item models.QuizItem
			if err := tx.Where("id = ? AND category_id = ?", itemID, categoryID).First(&item).Error; err != nil {
				return err
			}

			oldPos := item.Position
			catID := item.CategoryID

			// Відміняємо рендер, якщо він працює
			if cancelFunc, exists := activeRenderWorkers.Load(item.ID); exists {
				cancelFunc.(context.CancelFunc)()
			}

			media.CleanItemMedia(projectID, categoryID, itemID)

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

		parsedItemID, _ := strconv.ParseUint(itemID, 10, 32)
		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_deleted",
			ItemID: uint(parsedItemID),
		})

		c.JSON(http.StatusOK, gin.H{"message": "item deleted successfully"})
	}
}
