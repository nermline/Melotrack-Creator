package media

import (
	"fmt"
	"os"
	"path/filepath"
)

// CleanItemMedia видаляє конкретне оброблене відео та фото відповіді для QuizItem
func CleanItemMedia(projectID, categoryID, itemID string) {
	processedPath := filepath.Join(".", "downloads", "processed", projectID, categoryID, fmt.Sprintf("%s.mp4", itemID))
	_ = os.Remove(processedPath)

	imagePath := filepath.Join(".", "downloads", "answers", projectID, categoryID, fmt.Sprintf("%s.jpg", itemID))
	_ = os.Remove(imagePath)
}

// CleanCategoryMedia видаляє всі медіафайли, що належать конкретній категорії
func CleanCategoryMedia(projectID, categoryID string) {
	processedDir := filepath.Join(".", "downloads", "processed", projectID, categoryID)
	_ = os.RemoveAll(processedDir)

	answersDir := filepath.Join(".", "downloads", "answers", projectID, categoryID)
	_ = os.RemoveAll(answersDir)
}

// CleanProjectMedia видаляє абсолютно всі медіафайли проєкту
func CleanProjectMedia(projectID string) {
	processedDir := filepath.Join(".", "downloads", "processed", projectID)
	_ = os.RemoveAll(processedDir)

	answersDir := filepath.Join(".", "downloads", "answers", projectID)
	_ = os.RemoveAll(answersDir)
}
