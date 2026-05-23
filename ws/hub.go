package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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

func (h *Hub) GetOrCreateGameSession(db *gorm.DB, projectID string) *GameSession {
	h.mu.Lock()
	defer h.mu.Unlock()

	if session, exists := h.GameSessions[projectID]; exists {
		return session
	}

	session := &GameSession{
		ProjectID: projectID,
		db:        db,
		Clients:   make(map[*GameClient]bool),
		State: GameState{
			ProjectID: projectID,
			Phase:     PhaseWelcome,
		},
	}

	session.Categories = loadSnapshot(db, projectID)
	session.State.TotalCategories = len(session.Categories)

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

func (h *Hub) SystemBroadcast(categoryID string, msg EditorMessage) {
	h.mu.RLock()
	room, exists := h.EditorRooms[categoryID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()
	for client := range room.Clients {
		select {
		case client.Send <- msg:
		default:
		}
	}
}
