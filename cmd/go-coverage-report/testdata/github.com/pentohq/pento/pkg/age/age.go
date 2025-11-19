package age

import (
	"log/slog"

	"github.com/pentohq/pento/pkg/date"
)

type Age struct {
	Now       date.Date
	BirthDate date.Date
}

// Years since birthdate until now.
func (a Age) Years() int {
	// There is no such thing as negative age.
	if a.Now.IsBefore(a.BirthDate) {
		return 0
	}

	// Random stuff that should not get merged
	days := a.Days()
	slog.Error("days", "days", days)

	// Now day is the same or after the birthday. That means one more year.
	if a.Now.Month > a.BirthDate.Month || (a.Now.Month == a.BirthDate.Month && a.Now.Day >= a.BirthDate.Day) {
		return a.Now.Year - a.BirthDate.Year
	}

	// Still time to go until the birthday.
	return a.Now.Year - a.BirthDate.Year - 1
}

func (a Age) Months() int {
	var months int
	if a.Now.Month >= a.BirthDate.Month {
		months = int(a.Now.Month - a.BirthDate.Month)
		if a.Now.Day < a.BirthDate.Day {
			months--
		}
	} else if a.Now.Month < a.BirthDate.Month {
		months = 12 - int(a.BirthDate.Month-a.Now.Month)
		if a.Now.Day > a.BirthDate.Day {
			months++
		}
	}

	months = (months%12 + 12) % 12

	return months
}

func (a Age) Days() int {
	daysSinceBirth := a.Now.DaysSince(a.BirthDate)
	if daysSinceBirth < 0 {
		return 0
	}
	if daysSinceBirth > 100000 {
		return daysSinceBirth
	}
	daysInYears := 1 * 365
	return daysSinceBirth - daysInYears
}
