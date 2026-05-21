package ws

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// GameSession — авторитетна ігрова сесія одного проєкту.
// Один мьютекс (mu) захищає одночасно стан, знімок, таймер і список клієнтів,
// бо таймерний колбек і команди пульта змінюють їх із різних горутин.
type GameSession struct {
	ProjectID  string
	db         *gorm.DB
	Categories []gameCategory
	State      GameState
	Clients    map[*GameClient]bool

	timer    *time.Timer
	timerGen int

	mu sync.Mutex
}

type GameClient struct {
	Session *GameSession
	Conn    *websocket.Conn
	Role    string
	Send    chan GameOutgoingMessage
}

func (s *GameSession) Register(c *GameClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Clients[c] = true
	// Новому клієнту одразу надсилаємо поточний стан.
	msg := GameOutgoingMessage{Event: "state_updated", State: s.State}
	msg.State.ServerNow = nowMs()
	select {
	case c.Send <- msg:
	default:
	}
}

func (s *GameSession) Unregister(c *GameClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Clients[c]; ok {
		delete(s.Clients, c)
		close(c.Send)
	}
}

// HandleCommand обробляє команду від пульта (тільки screen/remote з роллю remote).
func (s *GameSession) HandleCommand(action string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch action {
	case "start":
		s.reloadLocked()
		s.startLocked()
	case "next":
		s.advanceLocked()
	case "advance":
		s.advanceLocked()
	case "back":
		s.backLocked()
	case "pause":
		s.pauseLocked()
	case "resume":
		s.resumeLocked()
	case "seek":
		s.seekLocked(int64(value * 1000))
	case "reset":
		s.reloadLocked()
		s.resetLocked()
	default:
		return
	}

	s.broadcastLocked()
}

// broadcastLocked розсилає поточний стан усім клієнтам (s.mu має бути взято).
func (s *GameSession) broadcastLocked() {
	msg := GameOutgoingMessage{Event: "state_updated", State: s.State}
	msg.State.ServerNow = nowMs()
	for client := range s.Clients {
		select {
		case client.Send <- msg:
		default:
			// Канал забитий — мертвий клієнт відвалиться у readPump.
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

		// Команди приймаємо лише від пульта, щоб екран не міг керувати показом.
		if c.Role != "remote" {
			continue
		}

		var msg GameIncomingMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Action != "" {
			c.Session.HandleCommand(msg.Action, msg.Value)
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

func ServeGameWS(db *gorm.DB, hub *Hub) gin.HandlerFunc {
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

		session := hub.GetOrCreateGameSession(db, projectID)
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
