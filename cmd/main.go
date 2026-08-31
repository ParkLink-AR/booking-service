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
	"github.com/example/bookings-service/internal/domain/constants"
	bookingrepository "github.com/example/bookings-service/internal/repository/booking"
	"github.com/example/bookings-service/internal/repository/messaging"
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

	// Services
	bookingService := bookingservice.NewService(bookingRepository, buenosAiresLocation, holdDuration())

	// Use cases
	listBookingsUseCase := ucbooking.NewListBookingsUseCase(bookingService)
	listBookingsByUserUseCase := ucbooking.NewListBookingsByUserUseCase(bookingRepository)
	createBookingUseCase := ucbooking.NewCreateBookingUseCase(bookingService)
	confirmBookingUseCase := ucbooking.NewConfirmBookingUseCase(bookingRepository)
	getBookableSpotsUseCase := ucbooking.NewGetBookableSpotsUseCase(bookingService)

	// Router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	loadHandlers(router, listBookingsUseCase, listBookingsByUserUseCase, createBookingUseCase, confirmBookingUseCase, getBookableSpotsUseCase, buenosAiresLocation)

	// Start consumer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Start(ctx)

	// Background job: expire PENDING_PAYMENT bookings whose hold has elapsed.
	// Without this, the DB's GIST EXCLUDE constraint keeps blocking those slots
	// even though the business layer considers them free, causing 409s on valid
	// new bookings.
	go runHoldExpiryJob(ctx, bookingRepository, logger)

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
	location *time.Location,
) {
	handlers := []Handler{
		bookingcontroller.NewHandler(listBookingsUseCase, listBookingsByUserUseCase, createBookingUseCase, confirmBookingUseCase, getBookableSpotsUseCase, location),
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
