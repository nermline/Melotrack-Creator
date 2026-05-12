package models

type Video struct {
	YouTubeURL string  `gorm:"not null"`
	StartTime  float64 `gorm:"not null;"`
	EndTime    float64 `gorm:"not null"`
	Volume     float64 `gorm:"default:1.0"`

	CropX      int
	CropY      int
	CropWidth  int
	CropHeight int
}
