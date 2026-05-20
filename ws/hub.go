package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client - це одне WebSocket з'єднання (екран або пульт)
type Client struct {
	Conn *websocket.Conn
	Role string // "screen" або "remote"
	Send chan []byte
}

// Session - це одна активна презентація
type Session struct {
	ProjectID string
	State     GameState
	Clients   map[*Client]bool
	mu        sync.RWMutex
}

// Hub керує всіма активними сесіями
type Hub struct {
	Sessions map[string]*Session
	mu       sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Sessions: make(map[string]*Session),
	}
}

// GetOrCreateSession повертає існуючу кімнату для проєкту або створює нову
func (h *Hub) GetOrCreateSession(projectID string) *Session {
	h.mu.Lock()
	defer h.mu.Unlock()

	if session, exists := h.Sessions[projectID]; exists {
		return session
	}

	session := &Session{
		ProjectID: projectID,
		State: GameState{
			ProjectID: projectID,
			Status:    "welcome", // Початковий стан за замовчуванням
		},
		Clients: make(map[*Client]bool),
	}
	h.Sessions[projectID] = session
	return session
}

// BroadcastState розсилає поточний стан усім підключеним клієнтам у сесії
func (s *Session) BroadcastState() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Тут ми будемо формувати OutgoingMessage та надсилати його в канал кожного клієнта
}
