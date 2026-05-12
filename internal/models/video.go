package models

type Video struct {
	YouTubeURL string

	MediaID *uint `gorm:"index"`
	Media   Media `gorm:"foreignKey:MediaID"`

	StartTime float64
	EndTime   float64
	Volume    float64 `gorm:"default:1.0"`

	CropX      int
	CropY      int
	CropWidth  int
	CropHeight int
}
