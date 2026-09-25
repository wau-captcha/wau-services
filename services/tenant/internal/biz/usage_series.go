package biz

import (
	"cmp"
	"slices"
)

func emptyDays(dates []string) []DailyUsage {
	days := make([]DailyUsage, len(dates))
	for i, date := range dates {
		days[i].Date = date
	}
	return days
}

func seriesFor(seriesByID map[string]*UsageSeries, id string, dates []string) *UsageSeries {
	series, ok := seriesByID[id]
	if !ok {
		series = &UsageSeries{ID: id, Days: emptyDays(dates)}
		seriesByID[id] = series
	}
	return series
}

func (s *UsageSeries) addDay(i int, count UsageCount) {
	s.addCount(count)
	s.Days[i].addCount(count)
}

// busiestFirst orders series by answers checked, then by challenges issued.
func busiestFirst(seriesByID map[string]*UsageSeries) []UsageSeries {
	sorted := make([]UsageSeries, 0, len(seriesByID))
	for _, series := range seriesByID {
		sorted = append(sorted, *series)
	}
	slices.SortFunc(sorted, func(left, right UsageSeries) int {
		return cmp.Or(
			cmp.Compare(right.Verifications, left.Verifications),
			cmp.Compare(right.Challenges, left.Challenges),
			cmp.Compare(left.ID, right.ID),
		)
	})
	return sorted
}
