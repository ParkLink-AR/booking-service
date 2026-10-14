package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	bookingcontroller "github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/controller/trustedcaller"
	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/repository/authclient"
	bookingrepository "github.com/example/bookings-service/internal/repository/booking"
	"github.com/example/bookings-service/internal/repository/messaging"
	"github.com/example/bookings-service/internal/repository/parkingclient"
	"github.com/example/bookings-service/internal/repository/paymentclient"
	"github.com/example/bookings-service/internal/repository/storage"
	bookingservice "github.com/example/bookings-service/internal/service/booking"
	ucbooking "github.com/example/bookings-service/internal/usecase/booking"
)

// holdExpiryInterval controls how often the background job expires stale holds.
// A value equal to the hold duration means a worst-case slot blockage of 2×hold.
// Running it more frequently (e.g. every minute) keeps the window tight.
const holdExpiryInterval = time.Minute

type Handler interface {
	RegisterRoutes(router chi.Router)
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Infrastructure
	database := setupDB()
	defer database.Close()

	rabbitConnection, rabbitChannel := setupRabbitMQ()
	defer rabbitConnection.Close()
	defer rabbitChannel.Close()

	buenosAiresLocation, err := time.LoadLocation(constants.BuenosAiresTimezone)
	if err != nil {
		log.Fatalf("load timezone: %v", err)
	}

	// Repositories
	bookingRepository := bookingrepository.NewRepository(database)
	consumer := messaging.NewConsumer(rabbitChannel, logger)
	eventPublisher := messaging.NewPublisher(rabbitChannel, rabbitExchangeName, logger)
	parkingClient := parkingclient.NewClient(envOrDefault("PARKING_SERVICE_URL", "http://localhost:3001"), os.Getenv("INTERNAL_SERVICE_KEY"))
	authClient := authclient.NewClient(envOrDefault("AUTH_SERVICE_URL", "http://localhost:8000"), os.Getenv("INTERNAL_SERVICE_KEY"))
	paymentClient := paymentclient.NewClient(
		envOrDefault("PAYMENT_SERVICE_URL", "http://localhost:8004"),
		os.Getenv("INTERNAL_SERVICE_KEY"),
	)
	storageClient, err := storage.NewClient(
		context.Background(),
		envOrDefault("R2_ACCOUNT_ID", ""),
		envOrDefault("R2_PAYOUT_PROOFS_BUCKET_NAME", ""),
		envOrDefault("R2_ACCESS_KEY_ID", ""),
		envOrDefault("R2_SECRET_ACCESS_KEY", ""),
	)
	if err != nil {
		log.Fatalf("build storage client: %v", err)
	}

	// Services
	bookingService := bookingservice.NewService(bookingRepository, parkingClient, bookingRepository, buenosAiresLocation, holdDuration())

	// Use cases
	listBookingsUseCase := ucbooking.NewListBookingsUseCase(bookingService)
	listBookingsByUserUseCase := ucbooking.NewListBookingsByUserUseCase(bookingRepository)
	createBookingUseCase := ucbooking.NewCreateBookingUseCase(bookingService, authClient)
	confirmBookingUseCase := ucbooking.NewConfirmBookingUseCase(bookingRepository, parkingClient, eventPublisher)
	getBookableSpotsUseCase := ucbooking.NewGetBookableSpotsUseCase(bookingService)
	approveBookingUseCase := ucbooking.NewApproveBookingUseCase(bookingRepository, parkingClient, eventPublisher)
	rejectBookingUseCase := ucbooking.NewRejectBookingUseCase(bookingRepository, parkingClient, paymentClient, eventPublisher)
	listBookingsDetailedUseCase := ucbooking.NewListBookingsDetailedUseCase(bookingRepository, parkingClient)
	getBookingUseCase := ucbooking.NewGetBookingUseCase(bookingRepository, parkingClient)
	submitReviewUseCase := ucbooking.NewSubmitReviewUseCase(bookingRepository, parkingClient, bookingRepository, parkingClient, eventPublisher)
	listReviewsByTargetUseCase := ucbooking.NewListReviewsByTargetUseCase(bookingRepository)
	listReviewsByBookingUseCase := ucbooking.NewListReviewsByBookingUseCase(bookingRepository)
	getSpotRatingsUseCase := ucbooking.NewGetSpotRatingsUseCase(bookingRepository)
	verifyBookingCodeUseCase := ucbooking.NewVerifyBookingCodeUseCase(bookingRepository, parkingClient, eventPublisher)
	cancelBookingUseCase := ucbooking.NewCancelBookingUseCase(bookingRepository, paymentClient, parkingClient, eventPublisher)
	sendMessageUseCase := ucbooking.NewSendMessageUseCase(bookingRepository, parkingClient, bookingRepository, eventPublisher)
	listMessagesUseCase := ucbooking.NewListMessagesUseCase(bookingRepository, parkingClient, bookingRepository)
	createTicketUseCase := ucbooking.NewCreateTicketUseCase(bookingRepository, parkingClient, bookingRepository, bookingRepository, eventPublisher)
	listMyTicketsUseCase := ucbooking.NewListMyTicketsUseCase(bookingRepository, parkingClient, bookingRepository)
	getTicketUseCase := ucbooking.NewGetTicketUseCase(bookingRepository)
	listTicketMessagesUseCase := ucbooking.NewListTicketMessagesUseCase(bookingRepository, bookingRepository)
	addTicketMessageUseCase := ucbooking.NewAddTicketMessageUseCase(bookingRepository, bookingRepository, bookingRepository, bookingRepository, eventPublisher)
	listAdminTicketsUseCase := ucbooking.NewListAdminTicketsUseCase(bookingRepository)
	getAdminTicketUseCase := ucbooking.NewGetAdminTicketUseCase(bookingRepository, bookingRepository, parkingClient)
	listAdminTicketMessagesUseCase := ucbooking.NewListAdminTicketMessagesUseCase(bookingRepository, bookingRepository)
	adminReplyUseCase := ucbooking.NewAdminReplyUseCase(bookingRepository, bookingRepository, bookingRepository, bookingRepository, eventPublisher)
	updateTicketStatusUseCase := ucbooking.NewUpdateTicketStatusUseCase(bookingRepository, bookingRepository, eventPublisher)
	generatePendingPayoutsUseCase := ucbooking.NewGeneratePendingPayoutsUseCase(bookingRepository, bookingRepository, bookingRepository, parkingClient)
	listAdminPayoutsUseCase := ucbooking.NewListAdminPayoutsUseCase(bookingRepository, storageClient)
	getAdminPayoutUseCase := ucbooking.NewGetAdminPayoutUseCase(bookingRepository, storageClient)
	listOwnerBookingsUseCase := ucbooking.NewListOwnerBookingsUseCase(parkingClient, bookingRepository)
	getCommissionUseCase := ucbooking.NewGetCommissionUseCase(bookingRepository)
	setCommissionUseCase := ucbooking.NewSetCommissionUseCase(bookingRepository)
	getOwnerPendingBalancesUseCase := ucbooking.NewGetOwnerPendingBalancesUseCase(bookingRepository)
	draftSettlementUseCase := ucbooking.NewDraftSettlementUseCase(bookingRepository, eventPublisher)
	confirmSettlementUseCase := ucbooking.NewConfirmSettlementUseCase(bookingRepository, bookingRepository, storageClient, storageClient, eventPublisher)
	cancelSettlementUseCase := ucbooking.NewCancelSettlementUseCase(bookingRepository)
	getSettlementUseCase := ucbooking.NewGetSettlementUseCase(bookingRepository, bookingRepository, storageClient)
	listAdminSettlementsUseCase := ucbooking.NewListAdminSettlementsUseCase(bookingRepository, storageClient)
	listSettlementsByOwnerUseCase := ucbooking.NewListSettlementsByOwnerUseCase(bookingRepository, storageClient)
	getOwnSettlementUseCase := ucbooking.NewGetOwnSettlementUseCase(bookingRepository, bookingRepository, storageClient)
	listTicketsByUserUseCase := ucbooking.NewListTicketsByUserUseCase(bookingRepository)
	listPendingVerificationReviewsUseCase := ucbooking.NewListPendingVerificationReviewsUseCase(bookingRepository)
	resolveVerificationReviewUseCase := ucbooking.NewResolveVerificationReviewUseCase(bookingRepository, paymentClient, parkingClient, eventPublisher)
	accountClosureUseCase := ucbooking.NewAccountClosureUseCase(bookingRepository, paymentClient, parkingClient, eventPublisher)
	expirePendingApprovalsUseCase := ucbooking.NewExpirePendingApprovalsUseCase(bookingRepository, paymentClient, parkingClient, eventPublisher)
	completeFinishedBookingsUseCase := ucbooking.NewCompleteFinishedBookingsUseCase(bookingRepository, parkingClient, eventPublisher)
	sendBookingRemindersUseCase := ucbooking.NewSendBookingRemindersUseCase(bookingRepository, parkingClient, eventPublisher)
	bookingChangesUseCase := ucbooking.NewBookingChangesUseCase(bookingRepository, bookingService, parkingClient, paymentClient, eventPublisher)
	adminOpsUseCase := ucbooking.NewAdminOpsUseCase(
		bookingRepository, paymentClient, bookingRepository, bookingRepository, bookingRepository, bookingRepository,
		parkingClient, eventPublisher, buenosAiresLocation,
	)

	// Router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(trustedcaller.Middleware(os.Getenv("GATEWAY_INTERNAL_SECRET"), os.Getenv("INTERNAL_SERVICE_KEY")))

	loadHandlers(
		router,
		listBookingsUseCase, listBookingsByUserUseCase, createBookingUseCase,
		confirmBookingUseCase, getBookableSpotsUseCase,
		approveBookingUseCase, rejectBookingUseCase, cancelBookingUseCase,
		listBookingsDetailedUseCase,
		getBookingUseCase,
		submitReviewUseCase, listReviewsByTargetUseCase, listReviewsByBookingUseCase,
		getSpotRatingsUseCase, verifyBookingCodeUseCase,
		sendMessageUseCase, listMessagesUseCase,
		createTicketUseCase, listMyTicketsUseCase, getTicketUseCase,
		listTicketMessagesUseCase, addTicketMessageUseCase,
		listAdminTicketsUseCase, getAdminTicketUseCase, listAdminTicketMessagesUseCase,
		adminReplyUseCase, updateTicketStatusUseCase,
		listAdminPayoutsUseCase, getAdminPayoutUseCase,
		listOwnerBookingsUseCase, getCommissionUseCase, setCommissionUseCase,
		getOwnerPendingBalancesUseCase, draftSettlementUseCase, confirmSettlementUseCase,
		cancelSettlementUseCase, getSettlementUseCase, listAdminSettlementsUseCase,
		listSettlementsByOwnerUseCase, getOwnSettlementUseCase,
		listTicketsByUserUseCase,
		buenosAiresLocation,
		os.Getenv("INTERNAL_SERVICE_KEY"),
		listPendingVerificationReviewsUseCase, resolveVerificationReviewUseCase,
		accountClosureUseCase,
		adminOpsUseCase,
		bookingChangesUseCase,
	)

	// Start consumer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Start(ctx)

	// Background job: expire PENDING_PAYMENT bookings whose hold has elapsed.
	// Without this, the DB's GIST EXCLUDE constraint keeps blocking those slots
	// even though the business layer considers them free, causing 409s on valid
	// new bookings.
	go runHoldExpiryJob(ctx, bookingRepository, logger)

	// Background job: auto-cancel + refund PENDING_OWNER_APPROVAL bookings the
	// owner never responded to within OWNER_APPROVAL_HOURS.
	go runOwnerApprovalExpiryJob(ctx, expirePendingApprovalsUseCase, logger)

	// Background job: mark CONFIRMED bookings COMPLETED once their time window has
	// passed -- this is what makes a stay "over" and triggers the review prompts.
	go runCompletionSweepJob(ctx, completeFinishedBookingsUseCase, logger)

	// Background job: turn every booking that's been COMPLETED and paid for at least 24h
	// into a PENDING payout row for ParkLink staff to review and mark paid.
	go runPayoutEligibilityJob(ctx, generatePendingPayoutsUseCase, logger)

	// Background job: reminders before a booking request expires unanswered, before a
	// booking starts and before a stay ends.
	go runRemindersJob(ctx, sendBookingRemindersUseCase, logger)

	port := envOrDefault("HTTP_PORT", constants.DefaultHTTPPort)
	server := &http.Server{Addr: ":" + port, Handler: router}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", zap.String("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	logger.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}

func loadHandlers(
	router chi.Router,
	listBookingsUseCase *ucbooking.ListBookingsUseCase,
	listBookingsByUserUseCase *ucbooking.ListBookingsByUserUseCase,
	createBookingUseCase *ucbooking.CreateBookingUseCase,
	confirmBookingUseCase *ucbooking.ConfirmBookingUseCase,
	getBookableSpotsUseCase *ucbooking.GetBookableSpotsUseCase,
	approveBookingUseCase *ucbooking.ApproveBookingUseCase,
	rejectBookingUseCase *ucbooking.RejectBookingUseCase,
	cancelBookingUseCase *ucbooking.CancelBookingUseCase,
	listBookingsDetailedUseCase *ucbooking.ListBookingsDetailedUseCase,
	getBookingUseCase *ucbooking.GetBookingUseCase,
	submitReviewUseCase *ucbooking.SubmitReviewUseCase,
	listReviewsByTargetUseCase *ucbooking.ListReviewsByTargetUseCase,
	listReviewsByBookingUseCase *ucbooking.ListReviewsByBookingUseCase,
	getSpotRatingsUseCase *ucbooking.GetSpotRatingsUseCase,
	verifyBookingCodeUseCase *ucbooking.VerifyBookingCodeUseCase,
	sendMessageUseCase *ucbooking.SendMessageUseCase,
	listMessagesUseCase *ucbooking.ListMessagesUseCase,
	createTicketUseCase *ucbooking.CreateTicketUseCase,
	listMyTicketsUseCase *ucbooking.ListMyTicketsUseCase,
	getTicketUseCase *ucbooking.GetTicketUseCase,
	listTicketMessagesUseCase *ucbooking.ListTicketMessagesUseCase,
	addTicketMessageUseCase *ucbooking.AddTicketMessageUseCase,
	listAdminTicketsUseCase *ucbooking.ListAdminTicketsUseCase,
	getAdminTicketUseCase *ucbooking.GetAdminTicketUseCase,
	listAdminTicketMessagesUseCase *ucbooking.ListAdminTicketMessagesUseCase,
	adminReplyUseCase *ucbooking.AdminReplyUseCase,
	updateTicketStatusUseCase *ucbooking.UpdateTicketStatusUseCase,
	listAdminPayoutsUseCase *ucbooking.ListAdminPayoutsUseCase,
	getAdminPayoutUseCase *ucbooking.GetAdminPayoutUseCase,
	listOwnerBookingsUseCase *ucbooking.ListOwnerBookingsUseCase,
	getCommissionUseCase *ucbooking.GetCommissionUseCase,
	setCommissionUseCase *ucbooking.SetCommissionUseCase,
	getOwnerPendingBalancesUseCase *ucbooking.GetOwnerPendingBalancesUseCase,
	draftSettlementUseCase *ucbooking.DraftSettlementUseCase,
	confirmSettlementUseCase *ucbooking.ConfirmSettlementUseCase,
	cancelSettlementUseCase *ucbooking.CancelSettlementUseCase,
	getSettlementUseCase *ucbooking.GetSettlementUseCase,
	listAdminSettlementsUseCase *ucbooking.ListAdminSettlementsUseCase,
	listSettlementsByOwnerUseCase *ucbooking.ListSettlementsByOwnerUseCase,
	getOwnSettlementUseCase *ucbooking.GetOwnSettlementUseCase,
	listTicketsByUserUseCase *ucbooking.ListTicketsByUserUseCase,
	location *time.Location,
	internalServiceKey string,
	listPendingVerificationReviewsUseCase *ucbooking.ListPendingVerificationReviewsUseCase,
	resolveVerificationReviewUseCase *ucbooking.ResolveVerificationReviewUseCase,
	accountClosureUseCase *ucbooking.AccountClosureUseCase,
	adminOpsUseCase *ucbooking.AdminOpsUseCase,
	bookingChangesUseCase *ucbooking.BookingChangesUseCase,
) {
	handlers := []Handler{
		bookingcontroller.NewHandler(
			listBookingsUseCase, listBookingsByUserUseCase, createBookingUseCase,
			confirmBookingUseCase, getBookableSpotsUseCase,
			approveBookingUseCase, rejectBookingUseCase, cancelBookingUseCase,
			listBookingsDetailedUseCase,
			getBookingUseCase,
			submitReviewUseCase, listReviewsByTargetUseCase, listReviewsByBookingUseCase,
			getSpotRatingsUseCase, verifyBookingCodeUseCase,
			sendMessageUseCase, listMessagesUseCase,
			createTicketUseCase, listMyTicketsUseCase, getTicketUseCase,
			listTicketMessagesUseCase, addTicketMessageUseCase,
			listAdminTicketsUseCase, getAdminTicketUseCase, listAdminTicketMessagesUseCase,
			adminReplyUseCase, updateTicketStatusUseCase,
			listAdminPayoutsUseCase, getAdminPayoutUseCase,
			listOwnerBookingsUseCase, getCommissionUseCase, setCommissionUseCase,
			getOwnerPendingBalancesUseCase, draftSettlementUseCase, confirmSettlementUseCase,
			cancelSettlementUseCase, getSettlementUseCase, listAdminSettlementsUseCase,
			listSettlementsByOwnerUseCase, getOwnSettlementUseCase,
			listTicketsByUserUseCase,
			location,
			internalServiceKey,
			listPendingVerificationReviewsUseCase, resolveVerificationReviewUseCase,
		),
		bookingcontroller.NewAccountClosureHandler(accountClosureUseCase, internalServiceKey),
		bookingcontroller.NewAdminOpsHandler(adminOpsUseCase, location),
		bookingcontroller.NewBookingChangesHandler(bookingChangesUseCase, location),
	}
	for _, handler := range handlers {
		handler.RegisterRoutes(router)
	}
}

// runHoldExpiryJob ticks every holdExpiryInterval and transitions every
// PENDING_PAYMENT row with an elapsed hold to EXPIRED, releasing the GIST
// EXCLUDE constraint so those time slots can be booked again.
func runHoldExpiryJob(ctx context.Context, repo *bookingrepository.Repository, logger *zap.Logger) {
	ticker := time.NewTicker(holdExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := repo.ExpireStaleHolds(ctx)
			if err != nil {
				logger.Error("hold expiry job failed", zap.Error(err))
			} else if n > 0 {
				logger.Info("expired stale holds", zap.Int64("count", n))
			}
		}
	}
}

// runOwnerApprovalExpiryJob ticks every minute and auto-cancels + refunds every
// PENDING_OWNER_APPROVAL booking whose owner_response_deadline has elapsed (see
// ExpirePendingApprovalsUseCase). Reuses the hold-expiry job's cadence -- these bookings
// are rare compared to payment holds, no need for a separate interval.
func runOwnerApprovalExpiryJob(ctx context.Context, useCase *ucbooking.ExpirePendingApprovalsUseCase, logger *zap.Logger) {
	ticker := time.NewTicker(holdExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := useCase.Execute(ctx)
			if err != nil {
				logger.Error("owner approval expiry job failed", zap.Error(err))
				continue
			}
			for _, bookingID := range result.RefundFailures {
				logger.Error("refund failed for expired pending-approval booking", zap.String("booking_id", bookingID))
			}
			if result.Expired > 0 {
				logger.Info("expired stale pending-owner-approval bookings", zap.Int("count", result.Expired))
			}
		}
	}
}

// runCompletionSweepJob ticks every minute and marks CONFIRMED bookings whose time
// window has already ended as COMPLETED (see CompleteFinishedBookingsUseCase).
func runCompletionSweepJob(ctx context.Context, useCase *ucbooking.CompleteFinishedBookingsUseCase, logger *zap.Logger) {
	ticker := time.NewTicker(holdExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := useCase.Execute(ctx)
			if err != nil {
				logger.Error("completion sweep job failed", zap.Error(err))
			} else if n > 0 {
				logger.Info("marked bookings completed", zap.Int("count", n))
			}
		}
	}
}

// runPayoutEligibilityJob ticks every holdExpiryInterval and turns every booking that's been
// COMPLETED and paid for at least 24h into a PENDING payout row -- see
// GeneratePendingPayoutsUseCase for why one booking's failure never blocks the rest.
func runPayoutEligibilityJob(ctx context.Context, useCase *ucbooking.GeneratePendingPayoutsUseCase, logger *zap.Logger) {
	ticker := time.NewTicker(holdExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			created, err := useCase.Execute(ctx)
			if err != nil {
				logger.Error("payout eligibility job failed", zap.Error(err))
			} else if created > 0 {
				logger.Info("generated pending payouts", zap.Int("count", created))
			}
		}
	}
}

// runRemindersJob ticks every holdExpiryInterval and sends the reminders that are due (see
// SendBookingRemindersUseCase).
func runRemindersJob(ctx context.Context, useCase *ucbooking.SendBookingRemindersUseCase, logger *zap.Logger) {
	ticker := time.NewTicker(holdExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sent, err := useCase.Execute(ctx)
			if err != nil {
				logger.Error("reminders job failed", zap.Error(err))
			}
			if sent > 0 {
				logger.Info("sent booking reminders", zap.Int("count", sent))
			}
		}
	}
}

// holdDuration reads BOOKING_HOLD_MINUTES, falling back to the default when unset or not
// a valid positive integer.
func holdDuration() time.Duration {
	raw := os.Getenv("BOOKING_HOLD_MINUTES")
	minutes := constants.DefaultHoldMinutes
	if raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			minutes = value
		}
	}
	return time.Duration(minutes) * time.Minute
}
