// Package parkingclient talks to parking-service to answer the things
// booking-service needs to know about a spot but doesn't own itself: who owns
// it, its human-facing title, and whether it auto-confirms bookings or requires
// the owner's approval.
package parkingclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

// spotResponse is the subset of parking-service's GET /parking-spots/{id}
// response this client cares about.
type spotResponse struct {
	OwnerID        string `json:"ownerId"`
	Title          string `json:"title"`
	InstantBooking bool   `json:"instantBooking"`
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

// GetSpotDetails returns the owner id, title and instant-booking flag of a parking spot
// in a single call. Used by the confirm/approve flows, which need all three: the owner to
// verify (approve) or notify (confirm), the title for the push copy, and the flag to
// decide whether a paid booking confirms immediately or waits on the owner.
func (client *Client) GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error) {
	spot, err := client.getSpot(ctx, spotID)
	if err != nil {
		return entity.SpotDetails{}, err
	}
	return entity.SpotDetails{
		OwnerID:        spot.OwnerID,
		Title:          spot.Title,
		InstantBooking: spot.InstantBooking,
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
