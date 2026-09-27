// Package live fans out real-time updates over one WebSocket per project:
// show state for screens and remotes, and content/score changes for editors.
package live

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Role string

const (
	RoleScreen Role = "screen" // presentation display: receives state, cannot control
	RoleRemote Role = "remote" // controls the show
	RoleEditor Role = "editor" // content editor / scoring: receives changes, may control
)

// ViewFunc renders the show state for a role; the hub marshals it once per role.
type ViewFunc func(projectID uint, host bool) (any, bool)

// CommandFunc handles a show command from a remote.
type CommandFunc func(projectID uint, action string, value float64)

type Hub struct {
	mu      sync.Mutex
	rooms   map[uint]map[*client]struct{}
	view    ViewFunc
	command CommandFunc
	origins []string

	upgrader websocket.Upgrader
}

func NewHub(allowedOrigins []string) *Hub {
	h := &Hub{rooms: map[uint]map[*client]struct{}{}, origins: allowedOrigins}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin,
	}
	return h
}

func (h *Hub) SetHandlers(view ViewFunc, command CommandFunc) {
	h.view, h.command = view, command
}

// checkOrigin allows same-host pages and explicitly configured origins (the dev server).
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	return slices.Contains(h.origins, origin)
}

type client struct {
	projectID uint
	role      Role
	conn      *websocket.Conn
	send      chan []byte
}

// Serve upgrades the request and keeps the connection until it closes.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, projectID uint, role Role) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{projectID: projectID, role: role, conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	if h.rooms[projectID] == nil {
		h.rooms[projectID] = map[*client]struct{}{}
	}
	h.rooms[projectID][c] = struct{}{}
	h.mu.Unlock()

	if h.view != nil {
		if v, ok := h.view(projectID, role != RoleScreen); ok {
			if data, err := json.Marshal(envelope{Type: "show", Data: v}); err == nil {
				c.send <- data
			}
		}
	}
	go h.writePump(c)
	go h.PublishShow(projectID) // remotes show which devices are connected
	h.readPump(c)
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[c.projectID]; ok {
		if _, ok := room[c]; ok {
			delete(room, c)
			close(c.send)
		}
		if len(room) == 0 {
			delete(h.rooms, c.projectID)
		}
	}
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
)

type incoming struct {
	Type   string  `json:"type"`
	Action string  `json:"action"`
	Value  float64 `json:"value"`
}

func (h *Hub) readPump(c *client) {
	defer func() {
		h.remove(c)
		c.conn.Close()
		go h.PublishShow(c.projectID)
	}()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		var msg incoming
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "cmd":
			if c.role != RoleScreen && h.command != nil {
				h.command(c.projectID, msg.Action, msg.Value)
			}
		case "ping":
			h.sendTo(c, envelope{Type: "pong", Data: time.Now().UnixMilli()})
		}
	}
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type envelope struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

func (h *Hub) sendTo(c *client, e envelope) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[c.projectID][c]; ok {
		trySend(c, data)
	}
}

// trySend never blocks: a client too slow to drain 64 messages is disconnected
// and will reconnect with a fresh state.
func trySend(c *client, data []byte) {
	select {
	case c.send <- data:
	default:
		go c.conn.Close()
	}
}

// Publish sends an event to everyone connected to the project.
func (h *Hub) Publish(projectID uint, typ string, data any) {
	payload, err := json.Marshal(envelope{Type: typ, Data: data})
	if err != nil {
		slog.Warn("live: marshal", "type", typ, "err", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.rooms[projectID] {
		if c.role == RoleScreen {
			continue // screens only care about the show
		}
		trySend(c, payload)
	}
}

// PublishShow pushes the current show state, rendered per role.
func (h *Hub) PublishShow(projectID uint) {
	if h.view == nil {
		return
	}
	h.mu.Lock()
	n := len(h.rooms[projectID])
	h.mu.Unlock()
	if n == 0 {
		return
	}
	var screen, host []byte
	if v, ok := h.view(projectID, false); ok {
		screen, _ = json.Marshal(envelope{Type: "show", Data: v})
	}
	if v, ok := h.view(projectID, true); ok {
		host, _ = json.Marshal(envelope{Type: "show", Data: v})
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.rooms[projectID] {
		if c.role == RoleScreen {
			if screen != nil {
				trySend(c, screen)
			}
		} else if host != nil {
			trySend(c, host)
		}
	}
}

// Connected lists how many devices of each role are connected (shown to the remote).
func (h *Hub) Connected(projectID uint) map[Role]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[Role]int{}
	for c := range h.rooms[projectID] {
		out[c.role]++
	}
	return out
}

// Rooms returns the ids of projects with at least one connection.
func (h *Hub) Rooms() []uint {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := make([]uint, 0, len(h.rooms))
	for id := range h.rooms {
		ids = append(ids, id)
	}
	return ids
}
