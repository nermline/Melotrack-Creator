// Package worker lets another machine download and cut videos for the main
// server. The main server sends jobs over HTTP (inside a VPN): the worker
// streams progress back as NDJSON and keeps source videos in a cache addressed
// by their sha256, so a source never has to be transferred twice.
//
//	GET    /v1/health
//	POST   /v1/download        {"youtube_id"}           → events, done.source = hash
//	HEAD   /v1/sources/{hash}                            → 200 when cached
//	GET    /v1/sources/{hash}                            → the source file
//	PUT    /v1/sources/{hash}  (file body)               → stores a source sent by the server
//	POST   /v1/render          {"source", "spec"}        → events, done.result = token
//	GET    /v1/results/{token}                           → the rendered clip
//	DELETE /v1/results/{token}
//
// Every request carries "Authorization: Bearer <WORKER_TOKEN>", except
// GET /healthz, which only tells Docker that the process is up.
package worker

import "github.com/nermline/Melotrack-Creator/internal/media"

type downloadRequest struct {
	YouTubeID string `json:"youtube_id"`
}

type renderRequest struct {
	Source string         `json:"source"`
	Spec   media.ClipSpec `json:"spec"`
}

// event is one NDJSON line of a job stream.
type event struct {
	Progress  *float64    `json:"progress,omitempty"`
	Heartbeat bool        `json:"heartbeat,omitempty"`
	Done      *doneEvent  `json:"done,omitempty"`
	Error     *errorEvent `json:"error,omitempty"`
}

type doneEvent struct {
	Source   string    `json:"source,omitempty"` // download: hash of the cached source
	Title    string    `json:"title,omitempty"`
	Loudness []float64 `json:"loudness,omitempty"`
	Result   string    `json:"result,omitempty"` // render: token of the clip
}

type errorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Health is what /v1/health reports.
type Health struct {
	OK           bool   `json:"ok"`
	Name         string `json:"name"`
	YtDlpVersion string `json:"ytdlp_version"`
	CachedBytes  int64  `json:"cached_bytes"`
}
