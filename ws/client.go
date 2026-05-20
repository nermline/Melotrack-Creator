package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Дозволяємо підключення з будь-якого Origin (корисно для розробки)
	},
}

type Client struct {
	Hub  *Hub
	Conn *websocket.Conn

	// Поля для ігрової сесії
	Session *Session
	Role    string
	Send    chan OutgoingMessage

	// Поля для кімнати редагування
	EditorRoom *EditorRoom
	EditorSend chan EditorMessage
}

func (c *Client) readPump() {
	defer func() {
		c.Session.mu.Lock()
		delete(c.Session.Clients, c)
		c.Session.mu.Unlock()
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var incomingMsg IncomingMessage
		if err := json.Unmarshal(message, &incomingMsg); err == nil {
			c.Session.HandleMessage(c, incomingMsg)
		}
	}
}

func (c *Client) writePump() {
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

// ServeWS обробляє вхідні WebSocket підключення
func ServeWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("pid")
		role := c.Query("role") // Очікується "screen" або "remote"

		if role != "screen" && role != "remote" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Use 'screen' or 'remote'"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade to websocket: %v", err)
			return
		}

		session := hub.GetOrCreateSession(projectID)

		client := &Client{
			Hub:     hub,
			Session: session,
			Conn:    conn,
			Role:    role,
			Send:    make(chan OutgoingMessage, 256),
		}

		session.mu.Lock()
		session.Clients[client] = true
		session.mu.Unlock()

		// Одразу надсилаємо клієнту поточний стан при підключенні
		client.Send <- OutgoingMessage{
			Event: "state_update",
			State: session.State,
		}

		go client.writePump()
		go client.readPump()
	}
}

func (c *Client) readEditorPump() {
	defer func() {
		c.EditorRoom.mu.Lock()
		delete(c.EditorRoom.Clients, c)
		c.EditorRoom.mu.Unlock()
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg EditorMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			// Якщо це чернетка редагування, миттєво ретранслюємо її іншим
			if msg.Action == "sync_edit" {
				c.EditorRoom.BroadcastEdit(c, msg)
			}
		}
	}
}

// Функція запису для кімнати редагування
func (c *Client) writeEditorPump() {
	defer c.Conn.Close()
	for msg := range c.EditorSend {
		if err := c.Conn.WriteJSON(msg); err != nil {
			break
		}
	}
}

// ServeEditorWS - ендпоінт для підключення організаторів до кімнати категорії
func ServeEditorWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		categoryID := c.Param("cid")

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to upgrade to websocket: %v", err)
			return
		}

		room := hub.GetOrCreateEditorRoom(categoryID)

		client := &Client{
			Conn:       conn,
			EditorRoom: room,
			EditorSend: make(chan EditorMessage, 256),
		}

		room.mu.Lock()
		room.Clients[client] = true
		room.mu.Unlock()

		go client.writeEditorPump()
		go client.readEditorPump()
	}
}
