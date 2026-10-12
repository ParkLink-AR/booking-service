package booking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
)

func TestCreateBookingValidationSuite(t *testing.T) {
	suite.Run(t, new(createBookingValidationSuite))
}

type createBookingValidationSuite struct {
	suite.Suite
}

// mustHour returns 2026-08-18 (a Tuesday, dayOfWeek=1) at hour, in location.
func mustHour(hour int) time.Time {
	return time.Date(2026, 8, 18, hour, 0, 0, 0, time.UTC)
}

const tuesday = 1

var isWithinPublishedHoursCases = []struct {
	name           string
	requestedStart int
	requestedEnd   int
	schedule       []entity.ScheduleEntry
	want           bool
}{
	{
		name:           "fully contained in a published window",
		requestedStart: 10,
		requestedEnd:   11,
		schedule:       []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 20}},
		want:           true,
	},
	{
		name:           "exactly matches the published window",
		requestedStart: 8,
		requestedEnd:   20,
		schedule:       []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 20}},
		want:           true,
	},
	{
		name:           "starts before the published window",
		requestedStart: 7,
		requestedEnd:   10,
		schedule:       []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 20}},
		want:           false,
	},
	{
		name:           "ends after the published window",
		requestedStart: 19,
		requestedEnd:   21,
		schedule:       []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 20}},
		want:           false,
	},
	{
		name:           "no schedule entries at all (closed day)",
		requestedStart: 10,
		requestedEnd:   11,
		schedule:       nil,
		want:           false,
	},
	{
		name:           "schedule entry exists for a different day of week",
		requestedStart: 10,
		requestedEnd:   11,
		schedule:       []entity.ScheduleEntry{{DayOfWeek: tuesday + 1, StartHour: 8, EndHour: 20}},
		want:           false,
	},
	{
		name:           "split shift: falls in the second window",
		requestedStart: 16,
		requestedEnd:   18,
		schedule: []entity.ScheduleEntry{
			{DayOfWeek: tuesday, StartHour: 8, EndHour: 14},
			{DayOfWeek: tuesday, StartHour: 16, EndHour: 22},
		},
		want: true,
	},
	{
		name:           "split shift: falls in the gap between windows",
		requestedStart: 14,
		requestedEnd:   16,
		schedule: []entity.ScheduleEntry{
			{DayOfWeek: tuesday, StartHour: 8, EndHour: 14},
			{DayOfWeek: tuesday, StartHour: 16, EndHour: 22},
		},
		want: false,
	},
}

// TestIsWithinPublishedHours_CrossesMidnight covers ranges that span more than one local day:
// every day's slice must sit inside that day's windows.
func (s *createBookingValidationSuite) TestIsWithinPublishedHours_CrossesMidnight() {
	wednesday := tuesday + 1
	allDay := func(day int) entity.ScheduleEntry {
		return entity.ScheduleEntry{DayOfWeek: day, StartHour: 0, EndHour: 24}
	}
	at := func(day, hour int) time.Time { return time.Date(2026, 8, day, hour, 0, 0, 0, time.UTC) }

	cases := []struct {
		name     string
		start    time.Time
		end      time.Time
		schedule []entity.ScheduleEntry
		want     bool
	}{
		{"23:00 to 01:00 with both days open all day", at(18, 23), at(19, 1), []entity.ScheduleEntry{allDay(tuesday), allDay(wednesday)}, true},
		{"23:00 to 01:00 but next day closed", at(18, 23), at(19, 1), []entity.ScheduleEntry{allDay(tuesday)}, false},
		{"23:00 to 01:00 but first day closes at 22", at(18, 23), at(19, 1), []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 22}, allDay(wednesday)}, false},
		{"23:00 to 01:00 but second day opens at 8", at(18, 23), at(19, 1), []entity.ScheduleEntry{allDay(tuesday), {DayOfWeek: wednesday, StartHour: 8, EndHour: 20}}, false},
		{"ends exactly at midnight stays on the first day", at(18, 22), at(19, 0), []entity.ScheduleEntry{{DayOfWeek: tuesday, StartHour: 8, EndHour: 24}}, true},
		{"three days with the middle one closed", at(18, 10), at(20, 10), []entity.ScheduleEntry{allDay(tuesday), allDay(wednesday + 1)}, false},
		{"two full days back to back", at(18, 0), at(20, 0), []entity.ScheduleEntry{allDay(tuesday), allDay(wednesday)}, true},
	}
	for _, testCase := range cases {
		got := isWithinPublishedHours(testCase.start, testCase.end, testCase.schedule, time.UTC)
		s.Equal(testCase.want, got, testCase.name)
	}
}

func (s *createBookingValidationSuite) TestIsWithinPublishedHours() {
	for _, testCase := range isWithinPublishedHoursCases {
		got := isWithinPublishedHours(mustHour(testCase.requestedStart), mustHour(testCase.requestedEnd), testCase.schedule, time.UTC)
		s.Equal(testCase.want, got, testCase.name)
	}
}
