package booking_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestSeedDemoDataUseCaseSuite(t *testing.T) {
	suite.Run(t, new(seedDemoDataUseCaseSuite))
}

type seedDemoDataUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	spots      *mocks.MockDemoSpotLister
	seeder     *mocks.MockDemoSpotSeeder
	useCase    *booking.SeedDemoDataUseCase
	context    context.Context
}

func (s *seedDemoDataUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.spots = mocks.NewMockDemoSpotLister(s.controller)
	s.seeder = mocks.NewMockDemoSpotSeeder(s.controller)
	s.useCase = booking.NewSeedDemoDataUseCase(s.spots, s.seeder)
	s.context = context.Background()
}

func (s *seedDemoDataUseCaseSuite) TearDownTest() { s.controller.Finish() }

func demoSpot(id string) entity.DemoSpot {
	price := 7500.0
	return entity.DemoSpot{ID: id, OwnerID: "0e000000-0000-4000-8000-000000000001", PricePerHour: &price}
}

func (s *seedDemoDataUseCaseSuite) TestLoadsEverySpotParkingListsAndSumsTheReviewsAdded() {
	s.spots.EXPECT().ListDemoSpots(s.context).Return([]entity.DemoSpot{demoSpot("a"), demoSpot("b")}, nil)
	s.seeder.EXPECT().SeedDemoSpot(s.context, gomock.Any()).Return(31, nil)
	s.seeder.EXPECT().SeedDemoSpot(s.context, gomock.Any()).Return(24, nil)

	result, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(booking.SeedDemoDataResult{Spots: 2, Reviews: 55}, result)
}

func (s *seedDemoDataUseCaseSuite) TestASpotThatIsAlreadyLoadedAddsNothing() {
	s.spots.EXPECT().ListDemoSpots(s.context).Return([]entity.DemoSpot{demoSpot("a")}, nil)
	s.seeder.EXPECT().SeedDemoSpot(s.context, gomock.Any()).Return(0, nil)

	result, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(booking.SeedDemoDataResult{Spots: 1, Reviews: 0}, result)
}

func (s *seedDemoDataUseCaseSuite) TestNoSpotsYetIsNotAnError() {
	s.spots.EXPECT().ListDemoSpots(s.context).Return(nil, nil)

	result, err := s.useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(booking.SeedDemoDataResult{}, result)
}

func (s *seedDemoDataUseCaseSuite) TestParkingBeingDownIsReportedSoTheJobRetries() {
	s.spots.EXPECT().ListDemoSpots(s.context).Return(nil, stderrors.New("connection refused"))

	_, err := s.useCase.Execute(s.context)

	s.Require().Error(err)
}

func (s *seedDemoDataUseCaseSuite) TestStopsAtTheFirstStoreFailureKeepingWhatWasAlreadyCounted() {
	s.spots.EXPECT().ListDemoSpots(s.context).Return([]entity.DemoSpot{demoSpot("a"), demoSpot("b"), demoSpot("c")}, nil)
	s.seeder.EXPECT().SeedDemoSpot(s.context, gomock.Any()).Return(30, nil)
	s.seeder.EXPECT().SeedDemoSpot(s.context, gomock.Any()).Return(0, stderrors.New("db down"))

	result, err := s.useCase.Execute(s.context)

	s.Require().Error(err)
	s.Equal(30, result.Reviews)
}

type demoSpotSeedSuite struct{ suite.Suite }

func TestDemoSpotSeedSuite(t *testing.T) { suite.Run(t, new(demoSpotSeedSuite)) }

var demoNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func (s *demoSpotSeedSuite) TestTheSameSpotAlwaysGetsTheSameReviews() {
	first := booking.BuildDemoSpotSeed(demoSpot("ff602e93-534d-5d48-b908-000000000001"), demoNow)
	second := booking.BuildDemoSpotSeed(demoSpot("ff602e93-534d-5d48-b908-000000000001"), demoNow)

	s.Equal(first, second)
}

func (s *demoSpotSeedSuite) TestEverySpotGetsBetweenTwentyAndFiftyReviewsByDifferentDrivers() {
	counts := map[int]bool{}
	for index := 0; index < 300; index++ {
		seed := booking.BuildDemoSpotSeed(demoSpot(fmt.Sprintf("spot-%d", index)), demoNow)

		s.GreaterOrEqual(len(seed.Reviews), constants.DemoReviewsMin)
		s.LessOrEqual(len(seed.Reviews), constants.DemoReviewsMax)
		counts[len(seed.Reviews)] = true

		reviewers := map[string]bool{}
		for _, review := range seed.Reviews {
			reviewers[review.ReviewerID] = true
		}
		s.Len(reviewers, len(seed.Reviews), "a driver reviewed the same spot twice")
	}
	s.Greater(len(counts), 20, "review counts should vary between spots")
}

func (s *demoSpotSeedSuite) TestReviewersAreTheExampleDrivers() {
	valid := map[string]bool{}
	for index := 1; index <= constants.DemoDriverCount; index++ {
		valid[fmt.Sprintf(constants.DemoDriverIDFormat, index)] = true
	}

	seed := booking.BuildDemoSpotSeed(demoSpot("abc"), demoNow)

	s.True(valid[seed.DriverID])
	for _, review := range seed.Reviews {
		s.True(valid[review.ReviewerID], review.ReviewerID)
	}
}

func (s *demoSpotSeedSuite) TestRatingsAreBelievableNotAllFives() {
	total, count := 0, 0
	seen := map[int]bool{}
	for index := 0; index < 200; index++ {
		for _, review := range booking.BuildDemoSpotSeed(demoSpot(fmt.Sprintf("spot-%d", index)), demoNow).Reviews {
			s.GreaterOrEqual(review.Rating, 1)
			s.LessOrEqual(review.Rating, 5)
			total += review.Rating
			count++
			seen[review.Rating] = true
		}
	}

	average := float64(total) / float64(count)
	s.InDelta(4.6, average, 0.15)
	s.True(seen[5] && seen[4] && seen[3], "should use more than one star count")
}

func (s *demoSpotSeedSuite) TestSomeReviewsHaveCommentsAndSomeOnlyStars() {
	withComment, without := 0, 0
	for index := 0; index < 50; index++ {
		for _, review := range booking.BuildDemoSpotSeed(demoSpot(fmt.Sprintf("spot-%d", index)), demoNow).Reviews {
			if review.Comment == "" {
				without++
			} else {
				withComment++
			}
		}
	}

	s.Greater(withComment, without/2)
	s.Greater(without, 0)
}

func (s *demoSpotSeedSuite) TestReviewsAreDatedInThePastAndSpreadOut() {
	seed := booking.BuildDemoSpotSeed(demoSpot("abc"), demoNow)

	days := map[int]bool{}
	for _, review := range seed.Reviews {
		s.True(review.CreatedAt.Before(demoNow))
		s.True(review.CreatedAt.After(demoNow.Add(-125 * 24 * time.Hour)))
		days[int(demoNow.Sub(review.CreatedAt).Hours()/24)] = true
	}
	s.Greater(len(days), 5)
	s.True(seed.End.Before(demoNow))
}

func (s *demoSpotSeedSuite) TestTheBookingIdIsStablePerSpotAndDifferentBetweenSpots() {
	a := booking.BuildDemoSpotSeed(demoSpot("spot-a"), demoNow)
	b := booking.BuildDemoSpotSeed(demoSpot("spot-b"), demoNow.Add(time.Hour))

	s.Equal(a.BookingID, booking.BuildDemoSpotSeed(demoSpot("spot-a"), demoNow.Add(48*time.Hour)).BookingID)
	s.NotEqual(a.BookingID, b.BookingID)
	s.Len(a.BookingID, 36)
}

func (s *demoSpotSeedSuite) TestTheBookingAmountIsTwoHoursOfTheSpotsPriceInCentavos() {
	seed := booking.BuildDemoSpotSeed(demoSpot("abc"), demoNow)
	s.Equal(7500*2*100, seed.AmountCents)

	noPrice := booking.BuildDemoSpotSeed(entity.DemoSpot{ID: "abc"}, demoNow)
	s.Equal(5000*2*100, noPrice.AmountCents)
}
