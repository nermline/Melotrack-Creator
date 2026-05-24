package handlers

import (
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"github.com/nermline/Melotrack-Creator/ws"
	"gorm.io/gorm"
)

func UploadAnswerImage(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
			return
		}
		defer src.Close()

		img, _, err := image.Decode(src)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported image format or corrupted file: %v", err)})
			return
		}

		outDir := filepath.Join(".", "downloads", "answers", projectID, categoryID)
		if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
			return
		}

		outPath := filepath.Join(outDir, fmt.Sprintf("%s.jpg", itemID))
		out, err := os.Create(outPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
			return
		}
		defer out.Close()

		var opt jpeg.Options
		opt.Quality = 85
		if err := jpeg.Encode(out, img, &opt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode image to jpeg"})
			return
		}

		webURL := fmt.Sprintf("/answers/%s/%s/%s.jpg", projectID, categoryID, itemID)

		if err := db.Model(&models.Answer{}).Where("quiz_item_id = ?", itemID).Update("image_path", webURL).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update database"})
			return
		}

		parsedItemID, _ := strconv.ParseUint(itemID, 10, 32)

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_image_updated",
			ItemID: uint(parsedItemID),
			Data:   map[string]string{"image_path": webURL},
		})

		c.JSON(http.StatusOK, gin.H{
			"message": "image uploaded successfully",
		})
	}
}

func DeleteQuizItemImage(db *gorm.DB, hub *ws.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		categoryID := c.Param("cid")
		itemID := c.Param("iid")

		if !categoryExists(c, db, projectID, categoryID) {
			return
		}

		imagePath := filepath.Join(".", "downloads", "answers", projectID, categoryID, fmt.Sprintf("%s.jpg", itemID))
		_ = os.Remove(imagePath)

		if err := db.Model(&models.Answer{}).Where("quiz_item_id = ?", itemID).Update("image_path", "").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update database"})
			return
		}

		parsedItemID, _ := strconv.ParseUint(itemID, 10, 32)

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_image_deleted",
			ItemID: uint(parsedItemID),
		})

		c.JSON(http.StatusOK, gin.H{"message": "answer image deleted successfully"})
	}
}
