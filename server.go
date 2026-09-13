package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/z3le/alien-buddy/content"
	"github.com/z3le/alien-buddy/display"
)

type Server struct {
	renderer *display.Renderer
	disp     display.Display
	rotation *content.Rotation
	start    time.Time
}

func NewServer(r *display.Renderer, d display.Display, rot *content.Rotation) *Server {
	return &Server{
		renderer: r,
		disp:     d,
		rotation: rot,
		start:    time.Now(),
	}
}

type PushMessage struct {
	Text string       `json:"text"`
	Mood content.Mood `json:"mood"`
}

func (s *Server) HandleMessage(w http.ResponseWriter, r *http.Request) {
	var msg PushMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if msg.Text == "" {
		http.Error(w, `{"error":"text is required"}`, http.StatusBadRequest)
		return
	}
	if msg.Mood == "" {
		msg.Mood = "happy"
	}

	img := s.renderer.Render(msg.Mood, msg.Text)
	if err := s.disp.Show(img); err != nil {
		log.Printf("display error: %v", err)
		http.Error(w, `{"error":"display failed"}`, http.StatusInternalServerError)
		return
	}

	// Update current state and pause rotation
	s.rotation.SetCurrent(msg.Text, msg.Mood)
	s.rotation.PauseFor(30 * time.Minute)

	log.Printf("pushed: mood=%s text=%q", msg.Mood, msg.Text)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) HandleCurrent(w http.ResponseWriter, r *http.Request) {
	current := s.rotation.Current()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(current)
}

func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	current := s.rotation.Current()
	uptime := time.Since(s.start).Round(time.Second)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"uptime":  uptime.String(),
		"current": current,
	})
}
