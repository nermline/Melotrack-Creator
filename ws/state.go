package ws

type GameState struct {
	ProjectID  string  `json:"project_id"`
	CategoryID uint    `json:"category_id"` // 0, якщо ще не почали
	ItemID     uint    `json:"item_id"`     // 0, якщо показується не відео
	Status     string  `json:"status"`
	SeekTime   float64 `json:"seek_time,omitempty"` // Використовується для перемотування
}

/* Можливі значення Status згідно з вашим алгоритмом:
- "welcome"    : Початковий екран (привітання/список категорій)
- "title"      : Показ назви категорії (1 секунда)
- "pre_item"   : 3-секундний відлік перед відео
- "playing"    : Відтворення відео/аудіо
- "thinking"   : 10-секундний таймер для команд
- "paused"     : Гра на паузі (відео або таймер зупинено)
- "answers"    : Слайд з правильними відповідями
*/

type IncomingMessage struct {
	Action   string  `json:"action"`    // "next", "pause", "play", "show_answers", "seek"
	TargetID uint    `json:"target_id"` // ID категорії або елемента, який треба увімкнути
	SeekTime float64 `json:"seek_time"` // Час для перемотування
}

type OutgoingMessage struct {
	Event string    `json:"event"` // "state_update"
	State GameState `json:"state"`
}
