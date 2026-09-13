package content

import "testing"

func TestLoadPhrases(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    map[string][]Phrase
		wantErr bool
	}{
		{
			name: "valid file groups by category and defaults mood",
			path: "testdata/phrases_valid.json",
			want: map[string][]Phrase{
				"morning": {
					{Text: "Good morning, Earthling!", Category: CategoryMorning, Mood: MoodHappy},
					{Text: "Rise and shine.", Category: CategoryMorning, Mood: MoodHappy},
				},
				"day": {
					{Text: "It is a fine day.", Category: CategoryDay, Mood: MoodExcited},
				},
				"evening": {
					{Text: "Good evening.", Category: CategoryEvening, Mood: MoodSleepy},
				},
			},
		},
		{
			name:    "empty array is an error",
			path:    "testdata/phrases_empty.json",
			wantErr: true,
		},
		{
			name:    "malformed json is an error",
			path:    "testdata/phrases_malformed.json",
			wantErr: true,
		},
		{
			name:    "missing file is an error",
			path:    "testdata/does_not_exist.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadPhrases(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("LoadPhrases(%q) error = nil, want error", tt.path)
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadPhrases(%q) unexpected error: %v", tt.path, err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("LoadPhrases(%q) = %d categories, want %d", tt.path, len(got), len(tt.want))
			}
			for cat, wantPhrases := range tt.want {
				gotPhrases, ok := got[cat]
				if !ok {
					t.Fatalf("LoadPhrases(%q) missing category %q", tt.path, cat)
				}
				if len(gotPhrases) != len(wantPhrases) {
					t.Fatalf("category %q = %d phrases, want %d", cat, len(gotPhrases), len(wantPhrases))
				}
				for i, wantPhrase := range wantPhrases {
					if gotPhrases[i] != wantPhrase {
						t.Errorf("category %q phrase[%d] = %+v, want %+v", cat, i, gotPhrases[i], wantPhrase)
					}
				}
			}
		})
	}
}
