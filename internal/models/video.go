package models

type Video struct {
	YouTubeURL    string  `json:"youtube_url"`
	MediaID       *uint   `gorm:"index" json:"-"`
	Media         Media   `gorm:"foreignKey:MediaID" json:"media"`
	StartTime     float64 `json:"start_time"`
	EndTime       float64 `json:"end_time"`
	Volume        float64 `gorm:"default:1.0" json:"volume"`
	CropX         int     `json:"crop_x"`
	CropY         int     `json:"crop_y"`
	CropWidth     int     `json:"crop_width"`
	CropHeight    int     `json:"crop_height"`
	// Fit=true → відео вписується у кадр 16:9 (letterbox), без втрати інформації.
	// Має пріоритет над crop.
	Fit           bool    `gorm:"not null;default:false" json:"fit"`
	RenderStatus  string  `gorm:"default:'unrendered'" json:"render_status"`
	ReadyFilePath string  `json:"-"` // Залишаємо прихованим (або видаляємо)
}
