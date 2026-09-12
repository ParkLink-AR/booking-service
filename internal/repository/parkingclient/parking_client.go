// Package parkingclient talks to parking-service to answer things
// booking-service needs to know about a spot but doesn't own itself: who owns
// it, whether it auto-confirms bookings or requires the owner's approval, and
// -- for a "stay" booking -- what it actually costs and whether the spot
// accepts it at all.
package parkingclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/errors"
)

// spotResponse is the subset of parking-service's GET /parking-spots/{id}
// response this client cares about.
type spotResponse struct {
	OwnerID        string `json:"ownerId"`
	InstantBooking bool   `json:"instantBooking"`
	Status         string `json:"status"`
}

// Client calls parking-service directly inside the docker network, the same
// way parking-service's own BookingClientService calls this service.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(baseURL string) *Client {
	return &Client{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: baseURL}
}

func (client *Client) getSpot(ctx context.Context, spotID string) (spotResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/parking-spots/%s", client.baseURL, spotID), nil)
	if err != nil {
		return spotResponse{}, errors.NewUpstream("parkingclient.getSpot", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return spotResponse{}, errors.NewUpstream("parkingclient.getSpot", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return spotResponse{}, errors.NewUpstream("parkingclient.getSpot", fmt.Errorf("unexpected status %d", response.StatusCode))
	}

	var spot spotResponse
	if err := json.NewDecoder(response.Body).Decode(&spot); err != nil {
		return spotResponse{}, errors.NewUpstream("parkingclient.getSpot", err)
	}
	return spot, nil
}

// IsInstantBooking reports whether a paid booking on this spot should confirm
// immediately (true) or wait on the owner's approval (false).
func (client *Client) IsInstantBooking(ctx context.Context, spotID string) (bool, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return false, err
	}
	return spot.InstantBooking, nil
}

// GetSpotOwnerID returns the owner id of a parking spot, used to verify that
// whoever is approving/rejecting a booking actually owns the spot.
func (client *Client) GetSpotOwnerID(ctx context.Context, spotID string) (string, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return "", err
	}
	return spot.OwnerID, nil
}

// IsSpotActive reports whether a parking spot currently accepts new reservations. A spot
// its owner paused (or deleted) keeps its existing bookings and detail page working --
// this only gates the creation of a new one (see Service.CreateBooking).
func (client *Client) IsSpotActive(ctx context.Context, spotID string) (bool, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return false, err
	}
	return spot.Status == "active", nil
}

// StayQuote mirrors parking-service's GET /parking-spots/{id}/stays/quote response: the
// canonical date range, exact check-in/check-out instants and price for a stay, computed
// and validated there (stayEnabled, stayAvailableFrom, maxStayDays, and overlap with
// existing bookings) -- never trusted from whatever the mobile client sends.
type StayQuote struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	// RFC3339 with the Buenos Aires offset -- the authoritative Start/End for the booking
	// this creates (see Service.priceStay), already carrying the driver's check-in hour on
	// both ends (a stay runs in 24h periods from check-in, not midnight to midnight).
	StartDateTime string `json:"startDateTime"`
	EndDateTime   string `json:"endDateTime"`
	TotalDays     int    `json:"totalDays"`
	// Amount is in centavos, same convention as entity.Booking.Amount.
	Amount int `json:"amount"`
}

// parkingServiceError is the shape of a NestJS exception response body
// (BadRequestException/NotFoundException/ConflictException all render this way).
type parkingServiceError struct {
	Message string `json:"message"`
}

// GetStayQuote asks parking-service to price and validate a stay -- startDate/endDate are
// YYYY-MM-DD, startHour (0-23) is the driver's chosen check-in hour in Buenos Aires time.
// The caller (CreateBooking) uses the returned StartDateTime/EndDateTime and Amount as-is
// instead of whatever the create-booking request carried, so a modified mobile client
// can't substitute a cheaper price or a shorter/longer range than what the spot's owner
// actually configured.
func (client *Client) GetStayQuote(ctx context.Context, spotID, startDate, endDate string, startHour int) (StayQuote, error) {
	query := url.Values{
		"startDate": {startDate},
		"endDate":   {endDate},
		"startHour": {strconv.Itoa(startHour)},
	}
	requestURL := fmt.Sprintf("%s/parking-spots/%s/stays/quote?%s", client.baseURL, spotID, query.Encode())

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return StayQuote{}, errors.NewUpstream("parkingclient.GetStayQuote", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return StayQuote{}, errors.NewUpstream("parkingclient.GetStayQuote", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		var quote StayQuote
		if err := json.NewDecoder(response.Body).Decode(&quote); err != nil {
			return StayQuote{}, errors.NewUpstream("parkingclient.GetStayQuote", err)
		}
		return quote, nil
	case http.StatusNotFound:
		return StayQuote{}, errors.NewNotFound("parking_spot", spotID)
	case http.StatusConflict:
		return StayQuote{}, errors.NewConflict(client.readErrorMessage(response, "requested stay overlaps with an existing booking"))
	case http.StatusBadRequest:
		return StayQuote{}, errors.NewInvalidInput("STAY_NOT_AVAILABLE", client.readErrorMessage(response, "this parking spot does not accept that stay"))
	default:
		return StayQuote{}, errors.NewUpstream("parkingclient.GetStayQuote", fmt.Errorf("unexpected status %d", response.StatusCode))
	}
}

// readErrorMessage best-efforts a human-readable reason out of a NestJS exception body,
// falling back to a generic message if the body isn't the shape we expect.
func (client *Client) readErrorMessage(response *http.Response, fallback string) string {
	var body parkingServiceError
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.Message == "" {
		return fallback
	}
	return body.Message
}
