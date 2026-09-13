package content

import (
	"testing"
	"time"
)

func TestScheduleCategoryAt(t *testing.T) {
	s := DefaultSchedule()

	tests := []struct {
		name string
		when time.Time
		want Category
	}{
		{"weekday before morning slot", time.Date(2026, 9, 15, 5, 0, 0, 0, time.UTC), CategoryNone},
		{"weekday morning slot start", time.Date(2026, 9, 15, 6, 0, 0, 0, time.UTC), CategoryMorning},
		{"weekday morning slot end is exclusive", time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC), CategoryNone},
		{"weekday midday has no category", time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC), CategoryNone},
		{"weekday evening slot", time.Date(2026, 9, 15, 21, 30, 0, 0, time.UTC), CategoryEvening},
		{"weekday after evening slot", time.Date(2026, 9, 15, 22, 0, 0, 0, time.UTC), CategoryNone},
		{"saturday day slot", time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC), CategoryDay},
		{"sunday morning slot", time.Date(2026, 9, 13, 7, 0, 0, 0, time.UTC), CategoryMorning},
		{"sunday evening slot", time.Date(2026, 9, 13, 21, 0, 0, 0, time.UTC), CategoryEvening},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.categoryAt(tt.when); got != tt.want {
				t.Errorf("categoryAt(%s) = %q, want %q", tt.when, got, tt.want)
			}
		})
	}
}

func TestScheduleGetCategoryReturnsAKnownCategory(t *testing.T) {
	s := DefaultSchedule()
	switch cat := s.GetCategory(); cat {
	case CategoryMorning, CategoryDay, CategoryEvening, CategoryNone:
	default:
		t.Errorf("GetCategory() = %q, want one of the defined categories", cat)
	}
}
