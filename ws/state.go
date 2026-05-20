package ws

// --- ДЛЯ ІГРОВОЇ СЕСІЇ (Екран та Пульт) ---

type GameState struct {
	ProjectID  string  `json:"project_id"`
	CategoryID uint    `json:"category_id"`
	ItemID     uint    `json:"item_id"`
	Status     string  `json:"status"`
	SeekTime   float64 `json:"seek_time,omitempty"`
}

type IncomingMessage struct {
	Action      string  `json:"action"`
	TargetState string  `json:"target_state,omitempty"`
	CategoryID  uint    `json:"category_id,omitempty"`
	ItemID      uint    `json:"item_id,omitempty"`
	SeekTime    float64 `json:"seek_time,omitempty"`
}

type OutgoingMessage struct {
	Event string     `json:"event"`
	State *GameState `json:"state,omitempty"` // Зверни увагу на зірочку (*)
}

// --- ДЛЯ КІМНАТИ РЕДАГУВАННЯ (Організатори) ---

type EditorMessage struct {
	Action string      `json:"action"`
	ItemID uint        `json:"item_id,omitempty"`
	Field  string      `json:"field,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}
