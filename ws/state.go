package ws

// --- ІГРОВА СЕСІЯ (Екран та Пульт) ---
//
// Сервер є авторитетом стану: він тримає єдиний стан гри, запускає таймери та
// сам переходить між фазами. Пульт (remote) надсилає лише команди, а екран
// (screen) рендерить отриманий стан. Завдяки цьому екран і пульт завжди
// синхронні, навіть якщо хтось перепідключиться посеред показу.

// Фази показу
const (
	PhaseWelcome       = "welcome"        // привітання + список категорій (ручний старт)
	PhaseCategoryTitle = "category_title" // показ назви категорії (таймер ~1с)
	PhaseCountdown     = "countdown"      // відлік перед питанням (таймер 3с)
	PhasePlaying       = "playing"        // відтворення відео/аудіо (таймер = довжина кліпу)
	PhaseThinking      = "thinking"       // час на роздуми (таймер 10с)
	PhaseAwaitAnswers  = "await_answers"  // після останнього питання, чекаємо команди показати відповіді
	PhaseAnswers       = "answers"        // слайд з відповідями категорії (ручний перехід далі)
	PhaseFinished      = "finished"       // кінець показу
)

// GameState — єдиний стан, який сервер розсилає всім клієнтам сесії.
type GameState struct {
	ProjectID string `json:"project_id"`
	Phase     string `json:"phase"`

	CategoryIndex int  `json:"category_index"`
	ItemIndex     int  `json:"item_index"`
	CategoryID    uint `json:"category_id"`
	ItemID        uint `json:"item_id"`

	ShowVideo bool `json:"show_video"` // показувати відео чи лише аудіо для поточного питання
	HasClip   bool `json:"has_clip"`   // чи є відрендерений кліп (render_status == ready)

	TotalCategories int `json:"total_categories"`
	TotalItems      int `json:"total_items"` // кількість питань у поточній категорії

	Paused bool `json:"paused"`

	// Тайминг поточної фази (мілісекунди, Unix epoch для PhaseEndsAt).
	// PhaseEndsAt == 0 означає фазу без таймера (ручний перехід або пауза).
	PhaseEndsAt   int64 `json:"phase_ends_at"`
	PhaseDuration int64 `json:"phase_duration_ms"`
	RemainingMs   int64 `json:"remaining_ms"`
	ServerNow     int64 `json:"server_now"` // час сервера на момент розсилки (для синхронізації годинників)
}

// GameIncomingMessage — команда від пульта.
//
// Action:
//   start    — почати показ з першої категорії (welcome → category_title)
//   pause    — поставити поточний таймер на паузу
//   resume   — продовжити з паузи
//   next     — ручний перехід далі (підтверджує await_answers, answers; з welcome = start)
//   advance  — примусовий перехід до наступної фази (оператор пропускає таймер)
//   back     — повернутися до попереднього питання
//   seek     — перемотати поточний таймер/відео на Value секунд (може бути від'ємним)
//   reset    — повернутися на екран привітання
type GameIncomingMessage struct {
	Action string  `json:"action"`
	Value  float64 `json:"value,omitempty"` // для seek: дельта в секундах
}

type GameOutgoingMessage struct {
	Event string    `json:"event"`
	State GameState `json:"state"`
}

// --- КІМНАТА РЕДАГУВАННЯ (Організатори) ---

type EditorMessage struct {
	Action string      `json:"action"`
	ItemID uint        `json:"item_id,omitempty"`
	Field  string      `json:"field,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}
