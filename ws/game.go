package ws

import (
	"encoding/json"
	"log"
	"time"

	"sync"

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
	Hub     *Hub
	Session *GameSession
	Conn    *websocket.Conn
	Role    string // "screen" або "remote"
	Send    chan OutgoingMessage
}

func (s *GameSession) BroadcastState() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msg := OutgoingMessage{
		Event: "state_update",
		State: &s.State,
	}

	for client := range s.Clients {
		select {
		case client.Send <- msg:
		default:
			close(client.Send)
			delete(s.Clients, client)
		}
	}
}

func (s *GameSession) HandleMessage(client *GameClient, msg IncomingMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.Action == "video_ended" && client.Role == "screen" {
		s.State.Status = "thinking"
		s.mu.Unlock()
		s.BroadcastState()
		s.mu.Lock()
		return
	}

	if client.Role == "remote" {
		stateChanged := false
		switch msg.Action {
		case "set_state":
			s.State.Status = msg.TargetState
			if msg.CategoryID != 0 {
				s.State.CategoryID = msg.CategoryID
			}
			if msg.ItemID != 0 {
				s.State.ItemID = msg.ItemID
			}
			s.State.SeekTime = 0
			stateChanged = true
		case "play":
			s.State.Status = "playing"
			stateChanged = true
		case "pause":
			s.State.Status = "paused"
			stateChanged = true
		case "seek":
			s.State.SeekTime = msg.SeekTime
			stateChanged = true
		}

		if stateChanged {
			s.mu.Unlock()
			s.BroadcastState()
			s.mu.Lock()
		}
	}
}

func (c *GameClient) readPump() {
	defer func() {
		c.Session.mu.Lock()
		delete(c.Session.Clients, c)
		c.Session.mu.Unlock()
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var incomingMsg IncomingMessage
		if err := json.Unmarshal(message, &incomingMsg); err == nil {
			c.Session.HandleMessage(c, incomingMsg)
		}
	}
}

func (c *GameClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteJSON(message)
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
			c.JSON(400, gin.H{"error": "Invalid role"})
			return
		}

		conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Upgrade error: %v", err)
			return
		}

		session := hub.GetOrCreateGameSession(projectID)

		client := &GameClient{
			Hub:     hub,
			Session: session,
			Conn:    conn,
			Role:    role,
			Send:    make(chan OutgoingMessage, 256),
		}

		session.mu.Lock()
		session.Clients[client] = true
		session.mu.Unlock()

		client.Send <- OutgoingMessage{Event: "state_update", State: &session.State}

		go client.writePump()
		go client.readPump()
	}
}
