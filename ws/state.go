package ws

const (
	PhaseWelcome       = "welcome"
	PhaseCategoryTitle = "category_title"
	PhaseCountdown     = "countdown"
	PhasePlaying       = "playing"
	PhaseThinking      = "thinking"
	PhaseAwaitAnswers  = "await_answers"
	PhaseAnswers       = "answers"
	PhaseFinished      = "finished"
)

type GameState struct {
	ProjectID string `json:"project_id"`
	Phase     string `json:"phase"`

	CategoryIndex int  `json:"category_index"`
	ItemIndex     int  `json:"item_index"`
	CategoryID    uint `json:"category_id"`
	ItemID        uint `json:"item_id"`

	ShowVideo bool `json:"show_video"`
	HasClip   bool `json:"has_clip"`

	TotalCategories int `json:"total_categories"`
	TotalItems      int `json:"total_items"`

	Paused bool `json:"paused"`

	PhaseEndsAt   int64 `json:"phase_ends_at"`
	PhaseDuration int64 `json:"phase_duration_ms"`
	RemainingMs   int64 `json:"remaining_ms"`
	ServerNow     int64 `json:"server_now"`
}

type GameIncomingMessage struct {
	Action string  `json:"action"`
	Value  float64 `json:"value,omitempty"`
}

type GameOutgoingMessage struct {
	Event string    `json:"event"`
	State GameState `json:"state"`
}

type EditorMessage struct {
	Action string      `json:"action"`
	ItemID uint        `json:"item_id,omitempty"`
	Field  string      `json:"field,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}
