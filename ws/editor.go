package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type EditorRoom struct {
	CategoryID string
	Clients    map[*EditorClient]bool
	mu         sync.RWMutex
}

type EditorClient struct {
	Room *EditorRoom
	Conn *websocket.Conn
	Send chan EditorMessage
}

func (r *EditorRoom) Register(c *EditorClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[c] = true
}

func (r *EditorRoom) Unregister(c *EditorClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Clients[c]; ok {
		delete(r.Clients, c)
		close(c.Send)
	}
}

func (r *EditorRoom) BroadcastEdit(sender *EditorClient, msg EditorMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for client := range r.Clients {
		if client != sender {
			select {
			case client.Send <- msg:
			default:
			}
		}
	}
}

func (c *EditorClient) readPump() {
	defer func() {
		c.Room.Unregister(c)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg EditorMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			// Пересилаємо чорнові зміни всім іншим у кімнаті
			if msg.Action == "sync_edit" {
				c.Room.BroadcastEdit(c, msg)
			}
		}
	}
}

func (c *EditorClient) writePump() {
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

func ServeEditorWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		categoryID := c.Param("cid")

		conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		room := hub.GetOrCreateEditorRoom(categoryID)
		client := &EditorClient{
			Room: room,
			Conn: conn,
			Send: make(chan EditorMessage, 256),
		}

		room.Register(client)

		go client.writePump()
		go client.readPump()
	}
}
