package models

type Video struct {
	YouTubeURL       string
	MediaID          *uint `gorm:"index" json:"-"`
	Media            Media `gorm:"foreignKey:MediaID" json:"-"`
	StartTime        float64
	EndTime          float64
	Volume           float64 `gorm:"default:1.0"`
	CropX            int
	CropY            int
	CropWidth        int
	CropHeight       int
	ProcessingStatus string `gorm:"default:'ready'"`
	ReadyFilePath    string
}
