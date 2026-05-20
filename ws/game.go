package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type GameSession struct {
	ProjectID string
	State     GameState
	Clients   map[*GameClient]bool
	mu        sync.RWMutex
}

type GameClient struct {
	Session *GameSession
	Conn    *websocket.Conn
	Role    string
	Send    chan GameOutgoingMessage
}

func (s *GameSession) Register(c *GameClient) {
	s.mu.Lock()
	s.Clients[c] = true
	s.mu.Unlock()

	// Відправляємо поточний стан тільки новому клієнту
	s.mu.RLock()
	stateCopy := s.State
	s.mu.RUnlock()

	c.Send <- GameOutgoingMessage{Event: "state_updated", State: stateCopy}
}

func (s *GameSession) Unregister(c *GameClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Clients[c]; ok {
		delete(s.Clients, c)
		close(c.Send)
	}
}

func (s *GameSession) UpdateState(newState GameState) {
	s.mu.Lock()
	s.State = newState
	s.mu.Unlock()
	s.Broadcast()
}

func (s *GameSession) Broadcast() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msg := GameOutgoingMessage{Event: "state_updated", State: s.State}

	for client := range s.Clients {
		select {
		case client.Send <- msg:
		default:
			// Якщо канал забитий, ігноруємо. Мертвий клієнт відвалиться в readPump
		}
	}
}

func (c *GameClient) readPump() {
	defer func() {
		c.Session.Unregister(c)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg GameIncomingMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg.Action == "update_state" {
				c.Session.UpdateState(msg.NewState)
			}
		}
	}
}

func (c *GameClient) writePump() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeGameWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		role := c.Query("role")

		if role != "screen" && role != "remote" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "role must be screen or remote"})
			return
		}

		conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		session := hub.GetOrCreateGameSession(projectID)
		client := &GameClient{
			Session: session,
			Conn:    conn,
			Role:    role,
			Send:    make(chan GameOutgoingMessage, 256),
		}

		session.Register(client)

		go client.writePump()
		go client.readPump()
	}
}
