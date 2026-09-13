package content

import (
	"fmt"
	"time"
)

type TimeSlot struct {
	Start    int
	End      int
	Category Category
}

type Schedule struct {
	Weekday []TimeSlot
	Weekend []TimeSlot
}

func DefaultSchedule() *Schedule {
	return &Schedule{
		// no day category during the weekdays cause there is nobody home
		Weekday: []TimeSlot{
			{Start: 6, End: 9, Category: CategoryMorning},
			{Start: 21, End: 22, Category: CategoryEvening},
		},
		Weekend: []TimeSlot{
			{Start: 6, End: 9, Category: CategoryMorning},
			{Start: 9, End: 21, Category: CategoryDay},
			{Start: 21, End: 22, Category: CategoryEvening},
		},
	}
}

func (s *Schedule) GetCategory() Category {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		fmt.Println("Error loading location:", err)
		loc = time.UTC // fallback to UTC if timezone cannot be loaded
	}

	return s.categoryAt(time.Now().In(loc))
}

func (s *Schedule) categoryAt(now time.Time) Category {
	hour := now.Hour()
	day := now.Weekday()

	var slots []TimeSlot
	if day == time.Saturday || day == time.Sunday {
		slots = s.Weekend
	} else {
		slots = s.Weekday
	}

	for _, slot := range slots {
		if hour >= slot.Start && hour < slot.End {
			return slot.Category
		}
	}

	return CategoryNone
}
