package content

import (
	"encoding/json"
	"fmt"
	"os"
)

type Phrase struct {
	Text     string   `json:"text"`
	Category Category `json:"category"`
	Mood     Mood     `json:"mood"`
}

type Mood string

const (
	MoodHappy    Mood = "happy"
	MoodExcited  Mood = "excited"
	MoodStern    Mood = "stern"
	MoodConfused Mood = "confused"
	MoodSleepy   Mood = "sleepy"
	MoodThinking Mood = "thinking"
)

type Category string

const (
	CategoryMorning Category = "morning"
	CategoryDay     Category = "day"
	CategoryEvening Category = "evening"
	CategoryNone    Category = ""
)

func LoadPhrases(path string) (map[string][]Phrase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw []Phrase
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("no phrases in %s", path)
	}

	phrases := make(map[string][]Phrase, len(raw))
	for _, p := range raw {
		if p.Mood == "" {
			p.Mood = MoodHappy
		}
		// group phrases by category so we can rotate depending on time of day
		phrases[string(p.Category)] = append(phrases[string(p.Category)], p)
	}

	return phrases, nil
}
