package content

import (
	"image"
	"log"
	"sync"
	"time"
)

// Renderer is the interface rotation uses to build images.
type Renderer interface {
	Render(mood Mood, text string) image.Image
}

// Display is the interface rotation uses to push images.
type Display interface {
	Show(img image.Image) error
	Clear() error
}

// CurrentMessage is what's on screen right now.
type CurrentMessage struct {
	Text     string   `json:"text"`
	Mood     Mood     `json:"mood"`
	Category Category `json:"category,omitempty"`
	PushedAt string   `json:"pushed_at,omitempty"`
	Source   string   `json:"source"` // "rotation" or "push"
}

type Rotation struct {
	phrases  map[string][]Phrase
	renderer Renderer
	display  Display
	interval time.Duration
	schedule *Schedule

	mu           sync.Mutex
	index        int
	lastCategory Category
	current      CurrentMessage
	paused       bool
	resumeAt     time.Time
	stopCh       chan struct{}
}

func NewRotation(phrases map[string][]Phrase, r Renderer, d Display, interval time.Duration) *Rotation {
	return &Rotation{
		phrases:  phrases,
		renderer: r,
		display:  d,
		interval: interval,
		schedule: DefaultSchedule(),
		stopCh:   make(chan struct{}),
	}
}

// Start begins rotating phrases on a timer.
func (rot *Rotation) Start() {
	// Show first phrase immediately
	rot.showNext()

	go func() {
		ticker := time.NewTicker(rot.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rot.mu.Lock()
				paused := rot.paused && time.Now().Before(rot.resumeAt)
				if paused {
					rot.mu.Unlock()
					continue
				}
				rot.paused = false
				rot.mu.Unlock()

				rot.showNext()

			case <-rot.stopCh:
				return
			}
		}
	}()
}

func (rot *Rotation) Stop() {
	close(rot.stopCh)
}

// PauseFor pauses rotation for the given duration (after a push message).
func (rot *Rotation) PauseFor(d time.Duration) {
	rot.mu.Lock()
	defer rot.mu.Unlock()
	rot.paused = true
	rot.resumeAt = time.Now().Add(d)
}

// Current returns what's on screen.
func (rot *Rotation) Current() CurrentMessage {
	rot.mu.Lock()
	defer rot.mu.Unlock()
	return rot.current
}

// SetCurrent is called by the HTTP handler when a push message is displayed.
func (rot *Rotation) SetCurrent(text string, mood Mood) {
	rot.mu.Lock()
	defer rot.mu.Unlock()
	rot.current = CurrentMessage{
		Text:     text,
		Mood:     mood,
		PushedAt: time.Now().Format(time.RFC3339),
		Source:   "push",
	}
}

func (rot *Rotation) showNext() {
	cat := rot.schedule.GetCategory()
	if cat == CategoryNone {
		log.Printf("rotation: no category for current time, clearing display")
		if err := rot.display.Clear(); err != nil {
			log.Printf("rotation clear display error: %v", err)
		}
		return
	}
	rot.mu.Lock()
	phrases, ok := rot.phrases[string(cat)]
	if !ok || len(phrases) == 0 {
		log.Printf("rotation: no phrases for category %q, clearing display", cat)
		if err := rot.display.Clear(); err != nil {
			log.Printf("rotation clear display error: %v", err)
		}
		rot.mu.Unlock()
		return
	}
	if cat != rot.lastCategory {
		rot.index = -1
		rot.lastCategory = cat
	}
	rot.index = (rot.index + 1) % len(phrases)
	p := phrases[rot.index]

	rot.current = CurrentMessage{
		Text:     p.Text,
		Mood:     p.Mood,
		Category: p.Category,
		Source:   "rotation",
	}
	rot.mu.Unlock()

	img := rot.renderer.Render(p.Mood, p.Text)
	if err := rot.display.Show(img); err != nil {
		log.Printf("rotation display error: %v", err)
	}
	log.Printf("rotation: [%s] %s", p.Mood, p.Text)
}
