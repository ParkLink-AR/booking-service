// Package parkingclient talks to parking-service to answer the things
// booking-service needs to know about a spot but doesn't own itself: who owns
// it, its human-facing title, whether it auto-confirms bookings or requires the
// owner's approval, and -- for a "stay" booking -- what it actually costs and
// whether the spot accepts it at all.
package parkingclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// spotResponse is the subset of parking-service's GET /parking-spots/{id}
// response this client cares about.
type spotResponse struct {
	OwnerID      string   `json:"ownerId"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	PricePerHour *float64 `json:"pricePerHour"`
	// Set when the hourly price depends on the vehicle's type -- PricePerHour is then nil.
	PricePerHourByVehicleType map[string]float64 `json:"pricePerHourByVehicleType"`
	// Only whether it's set matters here: parking-service prices the stay itself (see
	// GetStayQuote), by the vehicle type it's handed.
	StayPricesByVehicleType map[string]json.RawMessage `json:"stayPricesByVehicleType"`
	RequireVehicle          bool                       `json:"requireVehicle"`
	VehicleTypes            []string                   `json:"vehicleTypes"`
}

// Client calls parking-service directly inside the docker network, the same
// way parking-service's own BookingClientService calls this service.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	internalKey string
}

// NewClient builds a parking-service client. internalKey (INTERNAL_SERVICE_KEY) is sent on
// every call so parking-service accepts it as a trusted service-to-service request.
func NewClient(baseURL, internalKey string) *Client {
	return &Client{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: baseURL, internalKey: internalKey}
}

func (client *Client) newGetRequest(ctx context.Context, requestURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Internal-Key", client.internalKey)
	return request, nil
}

func (client *Client) getSpot(ctx context.Context, spotID string) (spotResponse, error) {
	request, err := client.newGetRequest(ctx, fmt.Sprintf("%s/parking-spots/%s", client.baseURL, spotID))
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

// GetSpotDetails returns the owner id and title of a parking spot in a single call. Used
// by the confirm/approve flows, which need the owner to verify (approve) or notify
// (confirm), and the title for the push copy.
func (client *Client) GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return entity.SpotDetails{}, err
	}
	return entity.SpotDetails{
		OwnerID: spot.OwnerID,
		Title:   spot.Title,
	}, nil
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

// ownedSpotResponse is the subset of parking-service's GET /users/{id}/parking-spots
// response this client cares about -- just enough to collect the owner's spot ids.
type ownedSpotResponse struct {
	ID string `json:"id"`
}

// GetSpotIDsForOwner returns the ids of every parking spot owned by ownerID, used to build
// GET /admin/owners/{id}/bookings -- the "reservations received across all of this owner's
// spots" view. Calls parking-service's existing public GET /users/{id}/parking-spots
// endpoint, same trust model as getSpot (no auth, same docker-network call).
func (client *Client) GetSpotIDsForOwner(ctx context.Context, ownerID string) ([]string, error) {
	request, err := client.newGetRequest(ctx, fmt.Sprintf("%s/users/%s/parking-spots", client.baseURL, ownerID))
	if err != nil {
		return nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", err)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", fmt.Errorf("unexpected status %d", response.StatusCode))
	}

	var spots []ownedSpotResponse
	if err := json.NewDecoder(response.Body).Decode(&spots); err != nil {
		return nil, errors.NewUpstream("parkingclient.GetSpotIDsForOwner", err)
	}

	ids := make([]string, len(spots))
	for i, spot := range spots {
		ids[i] = spot.ID
	}
	return ids, nil
}

// GetHourlyPrice returns the owner's own hourly rate for a spot (what the owner
// receives per hour, before the platform commission is added on top for the
// driver -- see Service.priceHourly). Returns an error if the spot has no
// hourly rate configured (stay-only spots have a nil pricePerHour).
//
// vehicleType picks the rate of a spot priced per vehicle type ("" when the driver named
// none -- Service.checkVehicle already requires one for such a spot).
func (client *Client) GetHourlyPrice(ctx context.Context, spotID, vehicleType string) (float64, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return 0, err
	}
	if spot.PricePerHourByVehicleType != nil {
		price, ok := spot.PricePerHourByVehicleType[vehicleType]
		if !ok || price <= 0 {
			return 0, errors.NewInvalidInput("VEHICLE_TYPE_NOT_ACCEPTED", "this parking spot has no hourly price for that type of vehicle")
		}
		return price, nil
	}
	if spot.PricePerHour == nil {
		return 0, errors.NewInvalidInput("HOURLY_NOT_AVAILABLE", "this parking spot does not accept hourly bookings")
	}
	return *spot.PricePerHour, nil
}

// GetVehicleRules returns what a spot asks of a booking's vehicle -- see
// entity.SpotVehicleRules. A spot saved before vehicle types existed fits cars only, same
// fallback parking-service applies.
func (client *Client) GetVehicleRules(ctx context.Context, spotID string) (entity.SpotVehicleRules, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return entity.SpotVehicleRules{}, err
	}
	accepted := spot.VehicleTypes
	if len(accepted) == 0 {
		accepted = []string{"car"}
	}
	return entity.SpotVehicleRules{
		RequireVehicle:        spot.RequireVehicle,
		HourlyPricedByVehicle: spot.PricePerHourByVehicleType != nil,
		StayPricedByVehicle:   spot.StayPricesByVehicleType != nil,
		AcceptedTypes:         accepted,
	}, nil
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
//
// vehicleType ("" when the driver named none) is what a spot that prices stays per vehicle
// type quotes by.
func (client *Client) GetStayQuote(ctx context.Context, spotID, startDate, endDate string, startHour int, vehicleType string) (StayQuote, error) {
	query := url.Values{
		"startDate": {startDate},
		"endDate":   {endDate},
		"startHour": {strconv.Itoa(startHour)},
	}
	if vehicleType != "" {
		query.Set("vehicleType", vehicleType)
	}
	requestURL := fmt.Sprintf("%s/parking-spots/%s/stays/quote?%s", client.baseURL, spotID, query.Encode())

	request, err := client.newGetRequest(ctx, requestURL)
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
