package booking

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/seed_demo_data_repository_mock.go -package=mocks -source=./seed_demo_data_usecase.go

// DemoSpotLister lists the demo parking spots. Implemented by internal/repository/parkingclient.
type DemoSpotLister interface {
	ListDemoSpots(ctx context.Context) ([]entity.DemoSpot, error)
}

// DemoSpotSeeder stores a demo spot's booking and reviews, skipping what is already there and
// returning how many reviews it added. Implemented by internal/repository/booking.
type DemoSpotSeeder interface {
	SeedDemoSpot(ctx context.Context, seed entity.DemoSpotSeed) (int, error)
}

// SeedDemoDataUseCase gives every demo parking spot between DemoReviewsMin and DemoReviewsMax
// reviews, so the app shows a rating and real-looking opinions on all of them. A review needs a
// booking, so each spot gets one finished booking (never paid, so it never reaches payouts) and
// all its reviews hang from it, written by different example drivers.
//
// Everything is derived from the spot's id: the same spot always gets the same reviews, and a
// run over spots that were already loaded adds nothing.
type SeedDemoDataUseCase struct {
	spots  DemoSpotLister
	seeder DemoSpotSeeder
	now    func() time.Time
}

func NewSeedDemoDataUseCase(spots DemoSpotLister, seeder DemoSpotSeeder) *SeedDemoDataUseCase {
	return &SeedDemoDataUseCase{spots: spots, seeder: seeder, now: time.Now}
}

// SeedDemoDataResult says how many demo spots parking-service listed and how many reviews this
// run added (0 when everything was already there).
type SeedDemoDataResult struct {
	Spots   int
	Reviews int
}

// Execute loads every listed demo spot.
func (uc *SeedDemoDataUseCase) Execute(ctx context.Context) (SeedDemoDataResult, error) {
	spots, err := uc.spots.ListDemoSpots(ctx)
	if err != nil {
		return SeedDemoDataResult{}, err
	}

	result := SeedDemoDataResult{Spots: len(spots)}
	for _, spot := range spots {
		inserted, err := uc.seeder.SeedDemoSpot(ctx, BuildDemoSpotSeed(spot, uc.now()))
		if err != nil {
			return result, errors.NewInternalError("SeedDemoDataUseCase.Execute", err)
		}
		result.Reviews += inserted
	}
	return result, nil
}

// demoNamespace derives the id of each spot's booking, so it is the same on every run.
var demoNamespace = uuid.MustParse("7e57de60-0000-4000-8000-00000000d3a0")

var positiveComments = []string{
	"Muy fácil de encontrar y el acceso es sencillo.",
	"Lugar limpio y seguro, volvería a reservar.",
	"El propietario respondió rápido y todo salió como estaba publicado.",
	"Buena ubicación y buen precio para la zona.",
	"Entré sin problemas, las indicaciones eran claras.",
	"Cómodo para ir al trabajo, lo uso seguido.",
	"Excelente atención, todo tal cual las fotos.",
	"Estacioné cerca de donde tenía que ir y salí sin vueltas.",
	"Muy buena experiencia, lo recomiendo.",
	"Amplio y bien señalizado, entra cómodo mi auto.",
	"Reservé a último momento y no tuve ningún inconveniente.",
	"Todo perfecto, el lugar estaba libre y listo.",
	"Buen lugar para dejar el auto durante el día.",
	"Fácil entrar y salir, sin maniobras complicadas.",
	"Me ahorró un montón de tiempo buscando lugar.",
	"Segura y tranquila la zona, sin sobresaltos.",
	"Muy práctico poder reservar desde la app.",
	"Cumple con lo que promete.",
	"Lo usé varias veces y siempre estuvo todo en orden.",
	"Lindo lugar, el propietario muy amable.",
}

var neutralComments = []string{
	"Está bien, aunque el acceso es un poco angosto.",
	"Cumple, pero tardaron un poco en abrirme.",
	"Correcto por el precio, nada especial.",
}

var criticalComments = []string{
	"Tuve que esperar un rato para poder entrar.",
	"Algo difícil de maniobrar para un auto grande.",
}

func hashOf(parts ...string) uint32 {
	hasher := fnv.New32a()
	for _, part := range parts {
		_, _ = hasher.Write([]byte(part))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum32()
}

func demoDriverID(index int) string {
	return fmt.Sprintf(constants.DemoDriverIDFormat, index%constants.DemoDriverCount+1)
}

// demoRating: about 68% five stars, 25% four, 6% three and 1% two (average near 4.6).
func demoRating(roll uint32) int {
	switch {
	case roll < 68:
		return 5
	case roll < 93:
		return 4
	case roll < 99:
		return 3
	default:
		return 2
	}
}

func demoComment(rating int, roll uint32) string {
	if roll%100 >= 65 {
		return "" // many people leave only the stars
	}
	switch rating {
	case 5, 4:
		return positiveComments[roll%uint32(len(positiveComments))]
	case 3:
		return neutralComments[roll%uint32(len(neutralComments))]
	default:
		return criticalComments[roll%uint32(len(criticalComments))]
	}
}

// BuildDemoSpotSeed is the booking and reviews a demo spot gets. Exposed for tests.
func BuildDemoSpotSeed(spot entity.DemoSpot, now time.Time) entity.DemoSpotSeed {
	count := constants.DemoReviewsMin + int(hashOf(spot.ID, "count")%uint32(constants.DemoReviewsMax-constants.DemoReviewsMin+1))
	firstDriver := int(hashOf(spot.ID, "driver") % constants.DemoDriverCount)

	reviews := make([]entity.DemoReview, count)
	for index := range reviews {
		roll := hashOf(spot.ID, "rating", fmt.Sprint(index))
		rating := demoRating(roll % 100)
		daysAgo := 1 + int(hashOf(spot.ID, "day", fmt.Sprint(index))%120)
		reviews[index] = entity.DemoReview{
			// 7 and 60 share no factor, so up to 60 reviews never repeat a reviewer.
			ReviewerID: demoDriverID(firstDriver + index*7),
			Rating:     rating,
			Comment:    demoComment(rating, hashOf(spot.ID, "comment", fmt.Sprint(index))),
			CreatedAt:  now.Add(-time.Duration(daysAgo)*24*time.Hour - time.Duration(hashOf(spot.ID, "hour", fmt.Sprint(index))%12)*time.Hour),
		}
	}

	pricePerHour := 5000.0
	if spot.PricePerHour != nil {
		pricePerHour = *spot.PricePerHour
	}
	start := now.Add(-130 * 24 * time.Hour).Truncate(time.Hour)

	return entity.DemoSpotSeed{
		BookingID:   uuid.NewSHA1(demoNamespace, []byte(spot.ID)).String(),
		Spot:        spot,
		DriverID:    demoDriverID(firstDriver),
		Start:       start,
		End:         start.Add(2 * time.Hour),
		AmountCents: int(pricePerHour * 2 * 100),
		Reviews:     reviews,
	}
}
