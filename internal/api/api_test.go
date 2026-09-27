package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/game"
	"github.com/nermline/Melotrack-Creator/internal/live"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
)

type env struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{DataDir: dir, StaticDir: filepath.Join(dir, "nope"), MaxUploadMB: 10, ClipHeight: 720}
	st, err := media.NewStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "t.db"), store.LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	// Tools that do not exist: downloads fail fast, nothing touches the network.
	yt := &media.YtDlp{Path: "/nonexistent/yt-dlp"}
	ff := &media.FFmpeg{FFmpeg: "/nonexistent/ffmpeg", FFprobe: "/nonexistent/ffprobe"}
	mgr := media.NewManager(db, st, yt, ff, nil, media.Options{DownloadWorkers: 1, RenderWorkers: 1, ClipHeight: 720})
	s := NewServer(db, cfg, auth.New("0123456789abcdef0123", "secret", time.Hour, false), mgr, yt, live.NewHub(nil), game.NewSessions(db))
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &env{t: t, srv: ts, client: &http.Client{Jar: jar}}
}

func (e *env) do(method, path string, body any, wantStatus int) map[string]any {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		e.t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, wantStatus, data)
	}
	var out map[string]any
	if len(data) > 0 && data[0] == '{' {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func (e *env) list(path string) []any {
	e.t.Helper()
	resp, err := e.client.Get(e.srv.URL + path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out []any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		e.t.Fatalf("GET %s: %v", path, err)
	}
	return out
}

func id(m map[string]any) int { return int(m["id"].(float64)) }

func TestAuth(t *testing.T) {
	e := newEnv(t)
	e.do("GET", "/api/projects", nil, http.StatusUnauthorized)
	e.do("GET", "/files/clips/x.mp4", nil, http.StatusUnauthorized)
	e.do("POST", "/api/login", map[string]string{"password": "wrong"}, http.StatusUnauthorized)
	e.do("POST", "/api/login", map[string]string{"password": "secret"}, http.StatusOK)
	e.do("GET", "/api/session", nil, http.StatusOK)
	e.do("POST", "/api/logout", nil, http.StatusOK)
	e.do("GET", "/api/session", nil, http.StatusUnauthorized)
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	for range 8 {
		e.do("POST", "/api/login", map[string]string{"password": "nope"}, http.StatusUnauthorized)
	}
	e.do("POST", "/api/login", map[string]string{"password": "secret"}, http.StatusTooManyRequests)
}

func TestContentFlow(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/login", map[string]string{"password": "secret"}, http.StatusOK)

	p := e.do("POST", "/api/projects", map[string]string{"title": "  Мелотрек   27.09 "}, http.StatusCreated)
	pid := id(p)
	if p["title"] != "Мелотрек 27.09" {
		t.Fatalf("title not normalised: %v", p["title"])
	}
	e.do("POST", "/api/projects", map[string]string{"title": "Мелотрек 27.09"}, http.StatusConflict)
	e.do("PATCH", fmt.Sprintf("/api/projects/%d", pid), map[string]any{"theme": "disco"}, http.StatusBadRequest)
	e.do("PATCH", fmt.Sprintf("/api/projects/%d", pid), map[string]any{"theme": "vinyl", "think_seconds": 15}, http.StatusOK)

	c1 := id(e.do("POST", fmt.Sprintf("/api/projects/%d/categories", pid), map[string]string{"title": "Хіти 2000-х"}, http.StatusCreated))
	c2 := id(e.do("POST", fmt.Sprintf("/api/projects/%d/categories", pid), map[string]string{"title": "Саундтреки"}, http.StatusCreated))
	e.do("PUT", fmt.Sprintf("/api/projects/%d/categories/order", pid), map[string]any{"ids": []int{c2, c1}}, http.StatusOK)
	e.do("PUT", fmt.Sprintf("/api/projects/%d/categories/order", pid), map[string]any{"ids": []int{c2}}, http.StatusConflict)

	res := e.do("POST", fmt.Sprintf("/api/categories/%d/items", c1), map[string]any{
		"urls": []string{"https://youtu.be/dQw4w9WgXcQ\nhttps://www.youtube.com/watch?v=9bZkp7q19f0&list=x\nnot a link"},
	}, http.StatusCreated)
	items := res["items"].([]any)
	if len(items) != 2 || len(res["invalid"].([]any)) != 1 {
		t.Fatalf("bulk add: %v", res)
	}
	first := items[0].(map[string]any)
	iid := id(first)
	if first["start"].(float64) != 0 || first["end"].(float64) != 15 || first["frame"] != "fit" {
		t.Fatalf("defaults: %v", first)
	}

	path := fmt.Sprintf("/api/items/%d", iid)
	e.do("PATCH", path, map[string]any{"start": 20, "end": 10}, http.StatusBadRequest)
	e.do("PATCH", path, map[string]any{"gain_db": 40}, http.StatusBadRequest)
	e.do("PATCH", path, map[string]any{"frame": "crop"}, http.StatusBadRequest)
	upd := e.do("PATCH", path, map[string]any{"answer": " Rick Astley — Never Gonna Give You Up ", "start": 42.04, "end": 57, "show_video": true}, http.StatusOK)
	if upd["answer"] != "Rick Astley — Never Gonna Give You Up" || upd["start"].(float64) != 42 || upd["show_video"] != true {
		t.Fatalf("patch: %v", upd)
	}
	e.do("PATCH", path, map[string]any{"youtube_url": "https://vimeo.com/1"}, http.StatusBadRequest)
	e.do("PATCH", path, map[string]any{"category_id": c2}, http.StatusOK)

	proj := e.do("GET", fmt.Sprintf("/api/projects/%d", pid), nil, http.StatusOK)
	cats := proj["categories"].([]any)
	if cats[0].(map[string]any)["title"] != "Саундтреки" {
		t.Fatalf("order not applied: %v", cats)
	}
	if n := len(cats[0].(map[string]any)["items"].([]any)); n != 1 {
		t.Fatalf("moved item missing, %d items", n)
	}
	if len(proj["media"].(map[string]any)) != 2 {
		t.Fatalf("media map: %v", proj["media"])
	}

	// Teams and scoring.
	t1 := id(e.do("POST", fmt.Sprintf("/api/projects/%d/teams", pid), map[string]string{"name": "Бітли"}, http.StatusCreated))
	t2 := id(e.do("POST", fmt.Sprintf("/api/projects/%d/teams", pid), map[string]string{"name": "Роллінги"}, http.StatusCreated))
	e.do("POST", fmt.Sprintf("/api/projects/%d/teams", pid), map[string]string{"name": "бітли"}, http.StatusConflict)
	e.do("PUT", "/api/scores", map[string]any{"team_id": t1, "item_id": iid, "points": 1}, http.StatusOK)
	e.do("PUT", "/api/scores", map[string]any{"team_id": t1, "item_id": iid, "points": 0.5}, http.StatusOK)
	e.do("PUT", "/api/scores", map[string]any{"team_id": t2, "item_id": iid, "points": 1}, http.StatusOK)
	e.do("PUT", "/api/scores", map[string]any{"team_id": t2, "item_id": iid, "points": 11}, http.StatusBadRequest)
	e.do("PATCH", fmt.Sprintf("/api/teams/%d", t1), map[string]any{"bonus": 2}, http.StatusOK)
	if s := e.list(fmt.Sprintf("/api/projects/%d/scores", pid)); len(s) != 2 {
		t.Fatalf("scores: %v", s)
	}

	// Public registration is closed until opened.
	code := proj["join_code"].(string)
	e.do("POST", "/api/public/join/"+code, map[string]string{"name": "Гості"}, http.StatusForbidden)
	e.do("PATCH", fmt.Sprintf("/api/projects/%d", pid), map[string]any{"registration_open": true}, http.StatusOK)
	anon := &http.Client{}
	resp, _ := anon.Post(e.srv.URL+"/api/public/join/"+code, "application/json", strings.NewReader(`{"name":"Гості"}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("public join: %d", resp.StatusCode)
	}
	resp, _ = anon.Post(e.srv.URL+"/api/public/join/"+code, "application/json", strings.NewReader(`{"name":"ГОСТІ"}`))
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate public join: %d", resp.StatusCode)
	}

	// Live show over WebSocket.
	ws := e.dialLive(pid, "remote")
	v := readShow(t, ws)
	if v["phase"] != "welcome" || len(v["teams"].([]any)) != 3 {
		t.Fatalf("welcome view: %v", v)
	}
	ws.WriteJSON(map[string]any{"type": "cmd", "action": "results"})
	v = waitPhase(t, ws, "scoring")
	ws.WriteJSON(map[string]any{"type": "cmd", "action": "next"})
	v = waitPhase(t, ws, "results")
	board := v["board"].([]any)
	top := board[0].(map[string]any)
	if top["name"] != "Бітли" || top["points"].(float64) != 2.5 {
		t.Fatalf("leaderboard: %v", board)
	}

	screen := e.dialLive(pid, "screen")
	sv := readShow(t, screen)
	if sv["host"] != nil {
		t.Fatal("screens must not receive host-only data")
	}
	screen.WriteJSON(map[string]any{"type": "cmd", "action": "reset"})
	time.Sleep(100 * time.Millisecond)
	if s := readShowNow(t, ws); s != nil && s["phase"] == "welcome" {
		t.Fatal("a screen must not be able to control the show")
	}

	e.do("DELETE", fmt.Sprintf("/api/items/%d", iid), nil, http.StatusOK)
	e.do("DELETE", fmt.Sprintf("/api/projects/%d", pid), nil, http.StatusOK)
	e.do("GET", fmt.Sprintf("/api/projects/%d", pid), nil, http.StatusNotFound)
}

func (e *env) dialLive(pid int, role string) *websocket.Conn {
	e.t.Helper()
	u, _ := url.Parse(e.srv.URL)
	wsURL := fmt.Sprintf("ws://%s/api/projects/%d/live?role=%s", u.Host, pid, role)
	hdr := http.Header{}
	for _, c := range e.client.Jar.Cookies(u) {
		hdr.Add("Cookie", c.String())
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		e.t.Fatalf("dial: %v", err)
	}
	e.t.Cleanup(func() { conn.Close() })
	return conn
}

func readShow(t *testing.T, ws *websocket.Conn) map[string]any {
	t.Helper()
	for {
		ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		var msg struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		}
		if err := ws.ReadJSON(&msg); err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.Type == "show" {
			return msg.Data
		}
	}
}

func readShowNow(t *testing.T, ws *websocket.Conn) map[string]any {
	ws.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var last map[string]any
	for {
		var msg struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		}
		if err := ws.ReadJSON(&msg); err != nil {
			return last
		}
		if msg.Type == "show" {
			last = msg.Data
		}
	}
}

func waitPhase(t *testing.T, ws *websocket.Conn, phase string) map[string]any {
	t.Helper()
	for range 10 {
		if v := readShow(t, ws); v["phase"] == phase {
			return v
		}
	}
	t.Fatalf("phase %s never arrived", phase)
	return nil
}
