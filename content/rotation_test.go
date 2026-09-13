package content

import (
	"errors"
	"image"
	"sync"
	"testing"
	"time"
)

type fakeRenderer struct {
	mu    sync.Mutex
	calls []renderCall
}

type renderCall struct {
	mood Mood
	text string
}

func (f *fakeRenderer) Render(mood Mood, text string) image.Image {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, renderCall{mood: mood, text: text})
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}

type fakeDisplay struct {
	mu         sync.Mutex
	shown      []image.Image
	clearCalls int
	showErr    error
	clearErr   error
}

func (f *fakeDisplay) Show(img image.Image) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, img)
	return f.showErr
}

func (f *fakeDisplay) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearCalls++
	return f.clearErr
}

func (f *fakeDisplay) shownCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.shown)
}

func newTestRotation(phrases map[string][]Phrase) (*Rotation, *fakeRenderer, *fakeDisplay) {
	r := &fakeRenderer{}
	d := &fakeDisplay{}
	rot := NewRotation(phrases, r, d, time.Hour)
	return rot, r, d
}

func alwaysCategory(cat Category) *Schedule {
	return &Schedule{
		Weekday: []TimeSlot{{Start: 0, End: 24, Category: cat}},
		Weekend: []TimeSlot{{Start: 0, End: 24, Category: cat}},
	}
}

func TestRotationShowNextNoCategoryClearsDisplay(t *testing.T) {
	rot, r, d := newTestRotation(map[string][]Phrase{
		"morning": {{Text: "hi", Mood: MoodHappy, Category: CategoryMorning}},
	})
	rot.schedule = alwaysCategory(CategoryNone)

	rot.showNext()

	if d.clearCalls != 1 {
		t.Errorf("clearCalls = %d, want 1", d.clearCalls)
	}
	if len(r.calls) != 0 {
		t.Errorf("render calls = %d, want 0", len(r.calls))
	}
	if got := rot.Current(); got != (CurrentMessage{}) {
		t.Errorf("Current() = %+v, want zero value", got)
	}
}

func TestRotationShowNextNoPhrasesForCategoryClearsDisplay(t *testing.T) {
	rot, _, d := newTestRotation(map[string][]Phrase{
		"morning": {{Text: "hi", Mood: MoodHappy, Category: CategoryMorning}},
	})
	rot.schedule = alwaysCategory(CategoryEvening)
	d.clearErr = errors.New("clear failed")

	rot.showNext()

	if d.clearCalls != 1 {
		t.Errorf("clearCalls = %d, want 1", d.clearCalls)
	}
}

func TestRotationShowNextCyclesThroughPhrases(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {
			{Text: "one", Mood: MoodHappy, Category: CategoryMorning},
			{Text: "two", Mood: MoodStern, Category: CategoryMorning},
			{Text: "three", Mood: MoodSleepy, Category: CategoryMorning},
		},
	}
	rot, _, _ := newTestRotation(phrases)
	rot.schedule = alwaysCategory(CategoryMorning)

	tests := []struct {
		call     int
		wantText string
	}{
		{1, "one"},
		{2, "two"},
		{3, "three"},
		{4, "one"},
	}

	for _, tt := range tests {
		rot.showNext()
		if got := rot.Current().Text; got != tt.wantText {
			t.Errorf("call %d: Current().Text = %q, want %q", tt.call, got, tt.wantText)
		}
	}
}

func TestRotationShowNextResetsIndexOnCategoryChange(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {
			{Text: "m1", Mood: MoodHappy, Category: CategoryMorning},
			{Text: "m2", Mood: MoodHappy, Category: CategoryMorning},
		},
		"evening": {
			{Text: "e1", Mood: MoodSleepy, Category: CategoryEvening},
			{Text: "e2", Mood: MoodSleepy, Category: CategoryEvening},
		},
	}
	rot, _, _ := newTestRotation(phrases)

	rot.schedule = alwaysCategory(CategoryMorning)
	rot.showNext()
	rot.showNext()
	if got := rot.Current().Text; got != "m2" {
		t.Fatalf("before switch: Current().Text = %q, want %q", got, "m2")
	}

	rot.schedule = alwaysCategory(CategoryEvening)
	rot.showNext()
	if got := rot.Current().Text; got != "e1" {
		t.Errorf("after switch: Current().Text = %q, want %q", got, "e1")
	}
}

func TestRotationShowNextSetsCurrentSource(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {{Text: "hi", Mood: MoodHappy, Category: CategoryMorning}},
	}
	rot, r, d := newTestRotation(phrases)
	rot.schedule = alwaysCategory(CategoryMorning)

	rot.showNext()

	want := CurrentMessage{Text: "hi", Mood: MoodHappy, Category: CategoryMorning, Source: "rotation"}
	if got := rot.Current(); got != want {
		t.Errorf("Current() = %+v, want %+v", got, want)
	}
	if len(d.shown) != 1 {
		t.Errorf("shown = %d, want 1", len(d.shown))
	}
	if len(r.calls) != 1 || r.calls[0] != (renderCall{mood: MoodHappy, text: "hi"}) {
		t.Errorf("render calls = %+v, want one call for the shown phrase", r.calls)
	}
}

func TestRotationShowNextSurvivesDisplayErrors(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {{Text: "hi", Mood: MoodHappy, Category: CategoryMorning}},
	}
	rot, _, d := newTestRotation(phrases)
	rot.schedule = alwaysCategory(CategoryMorning)
	d.showErr = errors.New("boom")

	rot.showNext()

	if got := rot.Current().Text; got != "hi" {
		t.Errorf("Current().Text = %q, want %q even when Show fails", got, "hi")
	}
}

func TestRotationSetCurrent(t *testing.T) {
	rot, _, _ := newTestRotation(nil)

	rot.SetCurrent("pushed message", MoodExcited)

	got := rot.Current()
	if got.Text != "pushed message" || got.Mood != MoodExcited || got.Source != "push" {
		t.Errorf("Current() = %+v, want pushed message with source push", got)
	}
	if got.PushedAt == "" {
		t.Error("PushedAt is empty, want a timestamp")
	}
}

func TestRotationPauseFor(t *testing.T) {
	rot, _, _ := newTestRotation(nil)

	rot.PauseFor(time.Minute)

	if !rot.paused {
		t.Error("paused = false, want true")
	}
	if !rot.resumeAt.After(time.Now()) {
		t.Error("resumeAt is not in the future")
	}
}

func TestRotationStartShowsFirstPhraseThenRotatesOnTicker(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {
			{Text: "one", Mood: MoodHappy, Category: CategoryMorning},
			{Text: "two", Mood: MoodStern, Category: CategoryMorning},
		},
	}
	r := &fakeRenderer{}
	d := &fakeDisplay{}
	rot := NewRotation(phrases, r, d, 5*time.Millisecond)
	rot.schedule = alwaysCategory(CategoryMorning)

	rot.Start()
	defer rot.Stop()

	deadline := time.Now().Add(time.Second)
	for d.shownCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if got := d.shownCount(); got < 3 {
		t.Fatalf("shown = %d after waiting, want at least 3", got)
	}
}

func TestRotationStartSkipsRotationWhilePaused(t *testing.T) {
	phrases := map[string][]Phrase{
		"morning": {{Text: "one", Mood: MoodHappy, Category: CategoryMorning}},
	}
	r := &fakeRenderer{}
	d := &fakeDisplay{}
	rot := NewRotation(phrases, r, d, 5*time.Millisecond)
	rot.schedule = alwaysCategory(CategoryMorning)
	rot.PauseFor(time.Hour)

	rot.Start()
	defer rot.Stop()

	time.Sleep(50 * time.Millisecond)

	if got := d.shownCount(); got != 1 {
		t.Errorf("shown = %d while paused, want 1 (only the initial showNext from Start)", got)
	}
}

func TestRotationStopClosesStopChannel(t *testing.T) {
	rot, _, _ := newTestRotation(nil)

	rot.Stop()

	select {
	case <-rot.stopCh:
	default:
		t.Error("stopCh was not closed")
	}
}
