package entity_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
)

func TestVehicleSuite(t *testing.T) {
	suite.Run(t, new(vehicleSuite))
}

type vehicleSuite struct {
	suite.Suite
}

type normalizeCase struct {
	input string
	want  string
}

var normalizePlateCases = []normalizeCase{
	{input: "ab123cd", want: "AB123CD"},
	{input: " ab 123-cd ", want: "AB123CD"},
	{input: "a.123.bcd", want: "A123BCD"},
	{input: "OHH827", want: "OHH827"},
}

var capitalizeWordsCases = []normalizeCase{
	{input: "honda", want: "Honda"},
	{input: "xr 250  tornado", want: "Xr 250 Tornado"},
	{input: "BMW", want: "BMW"},
	{input: "  ñandú  ", want: "Ñandú"},
	{input: "", want: ""},
}

func (s *vehicleSuite) TestNormalizePlate() {
	for _, testCase := range normalizePlateCases {
		s.Equal(testCase.want, entity.NormalizePlate(testCase.input), testCase.input)
	}
}

func (s *vehicleSuite) TestCapitalizeWords() {
	for _, testCase := range capitalizeWordsCases {
		s.Equal(testCase.want, entity.CapitalizeWords(testCase.input), testCase.input)
	}
}
