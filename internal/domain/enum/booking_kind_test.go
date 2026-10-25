package enum_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/enum"
)

func TestBookingKindSuite(t *testing.T) {
	suite.Run(t, new(bookingKindSuite))
}

type bookingKindSuite struct {
	suite.Suite
}

var bookingKindCases = []struct {
	label           string
	kind            enum.BookingKind
	tracksExit      bool
	usesStayPricing bool
}{
	{"HOURLY", enum.BookingKindHourly, true, false},
	{"STAY", enum.BookingKindStay, false, true},
	{"HALF_DAY", enum.BookingKindHalfDay, true, true},
}

func (s *bookingKindSuite) TestEachKind() {
	for _, testCase := range bookingKindCases {
		parsed, ok := enum.ParseBookingKind(testCase.label)
		s.True(ok, testCase.label)
		s.Equal(testCase.kind, parsed, testCase.label)
		s.Equal(testCase.label, testCase.kind.String(), testCase.label)
		s.Equal(testCase.tracksExit, testCase.kind.TracksExit(), testCase.label)
		s.Equal(testCase.usesStayPricing, testCase.kind.UsesStayPricing(), testCase.label)
	}
}

func (s *bookingKindSuite) TestUnknownKind() {
	_, ok := enum.ParseBookingKind("WEEKEND")
	s.False(ok)
}
