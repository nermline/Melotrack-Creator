package ws

// --- ДЛЯ ІГРОВОЇ СЕСІЇ (Екран та Пульт) ---

type GameState struct {
	ProjectID  string  `json:"project_id"`
	CategoryID uint    `json:"category_id"`
	ItemID     uint    `json:"item_id"`
	Status     string  `json:"status"` // "welcome", "title", "pre_item", "playing", "thinking", "paused", "answers"
	SeekTime   float64 `json:"seek_time,omitempty"`
}

type GameIncomingMessage struct {
	Action   string    `json:"action"` // "update_state", "ping"
	NewState GameState `json:"new_state"`
}

type GameOutgoingMessage struct {
	Event string    `json:"event"`
	State GameState `json:"state"`
}

// --- ДЛЯ КІМНАТИ РЕДАГУВАННЯ (Організатори) ---

type EditorMessage struct {
	Action string      `json:"action"`
	ItemID uint        `json:"item_id,omitempty"`
	Field  string      `json:"field,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}
