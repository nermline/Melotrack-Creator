package ws

import (
	"encoding/json"
	"log"

	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type EditorRoom struct {
	CategoryID string
	Clients    map[*EditorClient]bool
	mu         sync.RWMutex
}

type EditorClient struct {
	Hub  *Hub
	Room *EditorRoom
	Conn *websocket.Conn
	Send chan EditorMessage
}

func (r *EditorRoom) BroadcastEdit(sender *EditorClient, msg EditorMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for client := range r.Clients {
		if client != sender {
			select {
			case client.Send <- msg:
			default:
				close(client.Send)
				delete(r.Clients, client)
			}
		}
	}
}

// SystemBroadcast відправляє подію всім (включаючи ініціатора, наприклад, при видаленні елемента через HTTP)
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

func (c *EditorClient) readPump() {
	defer func() {
		c.Room.mu.Lock()
		delete(c.Room.Clients, c)
		c.Room.mu.Unlock()
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var msg EditorMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg.Action == "sync_edit" {
				c.Room.BroadcastEdit(c, msg)
			}
		}
	}
}

func (c *EditorClient) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteJSON(msg); err != nil {
			break
		}
	}
}

func ServeEditorWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		categoryID := c.Param("cid")

		conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Upgrade error: %v", err)
			return
		}

		room := hub.GetOrCreateEditorRoom(categoryID)

		client := &EditorClient{
			Hub:  hub,
			Room: room,
			Conn: conn,
			Send: make(chan EditorMessage, 256),
		}

		room.mu.Lock()
		room.Clients[client] = true
		room.mu.Unlock()

		go client.writePump()
		go client.readPump()
	}
}
