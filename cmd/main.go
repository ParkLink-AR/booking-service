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

	availabilitycontroller "github.com/example/bookings-service/internal/controller/availability"
	bookingcontroller "github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/domain/constants"
	bookingrepository "github.com/example/bookings-service/internal/repository/booking"
	"github.com/example/bookings-service/internal/repository/messaging"
	parkingservicerepository "github.com/example/bookings-service/internal/repository/parkingservice"
	availabilityservice "github.com/example/bookings-service/internal/service/availability"
	bookingservice "github.com/example/bookings-service/internal/service/booking"
	parkingservice "github.com/example/bookings-service/internal/service/parkingservice"
	ucavailability "github.com/example/bookings-service/internal/usecase/availability"
	ucbooking "github.com/example/bookings-service/internal/usecase/booking"
)

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
	parkingServiceRepository := parkingservicerepository.NewMockClient(buenosAiresLocation)
	consumer := messaging.NewConsumer(rabbitChannel, logger)

	// Services
	availabilityService := availabilityservice.NewService(bookingRepository, maxAvailabilityDays())
	parkingService := parkingservice.NewService(parkingServiceRepository)
	bookingService := bookingservice.NewService(bookingRepository, parkingService, buenosAiresLocation, holdDuration())

	// Use cases
	getAvailabilityUseCase := ucavailability.NewGetAvailabilityUseCase(parkingService, availabilityService)
	listBookingsUseCase := ucbooking.NewListBookingsUseCase(bookingService)
	createBookingUseCase := ucbooking.NewCreateBookingUseCase(bookingService)

	// Router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	loadHandlers(router, getAvailabilityUseCase, listBookingsUseCase, createBookingUseCase, buenosAiresLocation)

	// Start consumer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go consumer.Start(ctx)

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
	getAvailabilityUseCase *ucavailability.GetAvailabilityUseCase,
	listBookingsUseCase *ucbooking.ListBookingsUseCase,
	createBookingUseCase *ucbooking.CreateBookingUseCase,
	location *time.Location,
) {
	handlers := []Handler{
		availabilitycontroller.NewHandler(getAvailabilityUseCase, location),
		bookingcontroller.NewHandler(listBookingsUseCase, createBookingUseCase, location),
	}
	for _, handler := range handlers {
		handler.RegisterRoutes(router)
	}
}

// maxAvailabilityDays reads BOOKING_AVAILABILITY_MAX_DAYS, falling back to the default
// when unset or not a valid positive integer.
func maxAvailabilityDays() int {
	raw := os.Getenv("BOOKING_AVAILABILITY_MAX_DAYS")
	if raw == "" {
		return constants.DefaultMaxAvailabilityDays
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return constants.DefaultMaxAvailabilityDays
	}
	return value
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
