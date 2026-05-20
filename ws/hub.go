package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Для розробки. В продакшені краще вказати конкретні домени.
	},
}

// Hub керує всіма активними сесіями та кімнатами
type Hub struct {
	GameSessions map[string]*GameSession
	EditorRooms  map[string]*EditorRoom
	mu           sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		GameSessions: make(map[string]*GameSession),
		EditorRooms:  make(map[string]*EditorRoom),
	}
}

func (h *Hub) GetOrCreateGameSession(projectID string) *GameSession {
	h.mu.Lock()
	defer h.mu.Unlock()

	if session, exists := h.GameSessions[projectID]; exists {
		return session
	}

	session := &GameSession{
		ProjectID: projectID,
		State: GameState{
			ProjectID: projectID,
			Status:    "welcome",
		},
		Clients: make(map[*GameClient]bool),
	}
	h.GameSessions[projectID] = session
	return session
}

func (h *Hub) GetOrCreateEditorRoom(categoryID string) *EditorRoom {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.EditorRooms[categoryID]; exists {
		return room
	}

	room := &EditorRoom{
		CategoryID: categoryID,
		Clients:    make(map[*EditorClient]bool),
	}
	h.EditorRooms[categoryID] = room
	return room
}
