package ws

type GameState struct {
	ProjectID  string  `json:"project_id"`
	CategoryID uint    `json:"category_id"`         // 0, якщо ще не почали
	ItemID     uint    `json:"item_id"`             // 0, якщо показується не відео
	Status     string  `json:"status"`              // "welcome", "title", "pre_item", "playing", "thinking", "paused", "answers"
	SeekTime   float64 `json:"seek_time,omitempty"` // Для перемотування
}

type IncomingMessage struct {
	Action      string  `json:"action"`       // "set_state", "play", "pause", "seek", "video_ended"
	TargetState string  `json:"target_state"` // Для "set_state" (наприклад, "answers", "title")
	CategoryID  uint    `json:"category_id"`  // ID категорії, яку треба увімкнути
	ItemID      uint    `json:"item_id"`      // ID елемента (відео), який треба увімкнути
	SeekTime    float64 `json:"seek_time"`    // Час для перемотування
}

type OutgoingMessage struct {
	Event string    `json:"event"` // Завжди "state_update"
	State GameState `json:"state"`
}

type EditorMessage struct {
	Action string      `json:"action"`            // "sync_edit", "items_reordered", "item_deleted"
	ItemID uint        `json:"item_id,omitempty"` // Який елемент редагується
	Field  string      `json:"field,omitempty"`   // Наприклад: "volume", "start_time", "title"
	Value  interface{} `json:"value,omitempty"`   // Нове значення
	Data   interface{} `json:"data,omitempty"`    // Для масивів або складних об'єктів (наприклад, після видалення)
}
