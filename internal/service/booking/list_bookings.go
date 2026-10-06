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
//
// With activeOnly the result is the spot's *occupancy*, not the bookings as the owner or
// driver see them: each window is padded by the grace period on both sides (see
// entity.OccupancyWindow) and the [from, to) filter is widened to match, so a booking that
// ends just before from still shows up. Days are still bucketed by the booking's real start.
// This is what parking-service subtracts from a spot's published hours, which is how the
// grace period reaches the times a driver is offered without ever being shown to them.
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

	queryFrom, queryTo := from, to
	if activeOnly {
		queryFrom, queryTo = shiftBound(from, -entity.GracePeriod), shiftBound(to, entity.GracePeriod)
	}

	bookingWindows, err := service.bookingRepository.ListBookings(ctx, parkingSpotID, queryFrom, queryTo, status)
	if err != nil {
		return nil, err
	}
	if !activeOnly {
		return groupByDay(bookingWindows, service.location), nil
	}

	days := groupByDay(filterActiveWindows(bookingWindows), service.location)
	for dayIndex := range days {
		for windowIndex, window := range days[dayIndex].Bookings {
			occupancy := entity.OccupancyWindow(entity.TimeWindow{Start: window.Start, End: window.End})
			days[dayIndex].Bookings[windowIndex].Start = occupancy.Start
			days[dayIndex].Bookings[windowIndex].End = occupancy.End
		}
	}
	return days, nil
}

// shiftBound moves an optional range bound by delta; nil (unbounded) stays nil.
func shiftBound(bound *time.Time, delta time.Duration) *time.Time {
	if bound == nil {
		return nil
	}
	shifted := bound.Add(delta)
	return &shifted
}

// filterActiveWindows keeps only the bookings that currently occupy the spot (see isActive).
func filterActiveWindows(windows []entity.BookingWindow) []entity.BookingWindow {
	active := make([]entity.BookingWindow, 0, len(windows))
	for _, window := range windows {
		if isActive(window.Status, window.HoldExpiresAt) {
			active = append(active, window)
		}
	}
	return active
}
