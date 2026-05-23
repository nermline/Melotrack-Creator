package media

import (
	"fmt"
	"os"
	"path/filepath"
)

func CleanItemMedia(projectID, categoryID, itemID string) {
	processedPath := filepath.Join(".", "downloads", "processed", projectID, categoryID, fmt.Sprintf("%s.mp4", itemID))
	_ = os.Remove(processedPath)

	imagePath := filepath.Join(".", "downloads", "answers", projectID, categoryID, fmt.Sprintf("%s.jpg", itemID))
	_ = os.Remove(imagePath)
}

func CleanCategoryMedia(projectID, categoryID string) {
	processedDir := filepath.Join(".", "downloads", "processed", projectID, categoryID)
	_ = os.RemoveAll(processedDir)

	answersDir := filepath.Join(".", "downloads", "answers", projectID, categoryID)
	_ = os.RemoveAll(answersDir)
}

func CleanProjectMedia(projectID string) {
	processedDir := filepath.Join(".", "downloads", "processed", projectID)
	_ = os.RemoveAll(processedDir)

	answersDir := filepath.Join(".", "downloads", "answers", projectID)
	_ = os.RemoveAll(answersDir)
}
