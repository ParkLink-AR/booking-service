package constants

import "time"

// Demo data: reviews for the demo parking spots, loaded at startup when SEED_DEMO_DATA=true (see
// usecase.SeedDemoDataUseCase). Their reviewers are the example drivers auth-service creates with
// the same fixed ids (auth-service/app/demo_seed.py) -- this service never asks auth for them.
const (
	// DemoDriverCount is how many example drivers auth-service creates.
	DemoDriverCount = 60
	// DemoDriverIDFormat builds the id of example driver n (1..DemoDriverCount).
	DemoDriverIDFormat = "de000000-0000-4000-8000-%012d"

	// DemoReviewsMin and DemoReviewsMax bound how many reviews each demo spot gets.
	DemoReviewsMin = 20
	DemoReviewsMax = 50

	// DemoSeedRetryInterval and DemoSeedMaxAttempts: parking-service may still be starting (or
	// still completing its spots) when this one boots, so the load is retried a few times.
	DemoSeedRetryInterval = 30 * time.Second
	DemoSeedMaxAttempts   = 20
)
