package models

import "time"

type Answer struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	QuizItemID uint      `gorm:"not null" json:"quiz_item_id"`
	Title      string    `json:"title"`
	// ImagePath зберігає web-URL (/answers/{pid}/{cid}/{iid}.jpg) — безпечно відкривати
	ImagePath  string    `json:"image_path,omitempty"`
}
