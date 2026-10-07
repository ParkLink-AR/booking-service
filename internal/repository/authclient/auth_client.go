// Package authclient asks auth-service about a user's account -- e.g. whether it can still
// make bookings (not while it's suspended or being deleted).
package authclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/errors"
)

// Client calls auth-service's internal endpoints directly on the docker network with the
// shared internal key, the same way parkingclient.Client calls parking-service.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	internalKey string
}

func NewClient(baseURL, internalKey string) *Client {
	return &Client{httpClient: &http.Client{Timeout: 10 * time.Second}, baseURL: baseURL, internalKey: internalKey}
}

type accountStatusResponse struct {
	Status string `json:"status"`
}

// GetAccountStatus returns the account's status as auth-service names it (ACTIVE,
// SUSPENDED, PENDING_DELETION, DELETED).
func (client *Client) GetAccountStatus(ctx context.Context, userID string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/internal/users/"+userID+"/account-status", nil)
	if err != nil {
		return "", errors.NewUpstream("authclient.GetAccountStatus", err)
	}
	request.Header.Set("X-Internal-Key", client.internalKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", errors.NewUpstream("authclient.GetAccountStatus", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", errors.NewUpstream("authclient.GetAccountStatus", fmt.Errorf("unexpected status %d", response.StatusCode))
	}
	var body accountStatusResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", errors.NewUpstream("authclient.GetAccountStatus", err)
	}
	return body.Status, nil
}
