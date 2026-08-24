package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// ListBookings returns the parking spot's bookings grouped by calendar day (in the
// service's location), ordered chronologically. Only days with at least one booking appear.
func (service *Service) ListBookings(
	ctx context.Context,
	parkingSpotID string,
	from, to *time.Time,
	status *enum.BookingStatus,
	activeOnly bool,
) ([]entity.DayBookings, error) {
	if from != nil && to != nil && !from.Before(*to) {
		return nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to")
	}

	bookingWindows, err := service.bookingRepository.ListBookings(ctx, parkingSpotID, from, to, status)
	if err != nil {
		return nil, err
	}
	if activeOnly {
		bookingWindows = filterActiveWindows(bookingWindows)
	}

	return groupByDay(bookingWindows, service.location), nil
}

// filterActiveWindows keeps only the bookings that currently occupy the spot.
func filterActiveWindows(windows []entity.BookingWindow) []entity.BookingWindow {
	active := make([]entity.BookingWindow, 0, len(windows))
	for _, window := range windows {
		if isActive(window.Status, window.HoldExpiresAt) {
			active = append(active, window)
		}
	}
	return active
}
