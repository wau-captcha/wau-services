package biz

import (
	"time"

	"tenant/internal/conf"

	"common/quota"
)

// Calendar is the local calendar that usage charts and the daily quota follow, so a
// chart's "today" and the quota's "today" are always the same day.
type Calendar struct {
	location *time.Location
	zone     string
	now      func() time.Time
}

func NewCalendar(usageConf *conf.Usage) (*Calendar, error) {
	zone := usageConf.GetTimeZone()
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	return &Calendar{location: location, zone: zone, now: time.Now}, nil
}

// Zone is the IANA name of the time zone.
func (c *Calendar) Zone() string { return c.zone }

// Today is the local date, YYYY-MM-DD.
func (c *Calendar) Today() string { return quota.Day(c.now(), c.location) }

// NextReset is the next local midnight, when the daily counters start over.
func (c *Calendar) NextReset() time.Time { return quota.NextReset(c.now(), c.location) }

// WindowStart is the local midnight that opens a window of `days` days ending today.
func (c *Calendar) WindowStart(days int) time.Time {
	today := c.now().In(c.location)
	return time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, c.location).AddDate(0, 0, -(days - 1))
}

// DatesFrom lists the local dates from first to today, YYYY-MM-DD and oldest first. It is just
// today when first is today, later, or not a date.
func (c *Calendar) DatesFrom(first string) []string {
	today := c.Today()
	day, err := time.ParseInLocation(time.DateOnly, first, c.location)
	if err != nil || first >= today {
		return []string{today}
	}
	var dates []string
	for date := first; ; {
		dates = append(dates, date)
		if date >= today {
			return dates
		}
		day = day.AddDate(0, 0, 1)
		date = day.Format(time.DateOnly)
	}
}
