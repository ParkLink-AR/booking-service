// Package paymentclient triggers a refund on payment-service when a booking
// that was already paid gets rejected (or expires unanswered) by the owner.
package paymentclient

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/errors"
)

// Client calls payment-service's own internal endpoint directly, the same
// way parkingclient.Client calls parking-service. Not routed through the
// gateway -- POST /internal/refund-by-payment isn't gateway-registered, so
// it needs the shared internal key instead (payment-service's port is
// reachable directly, same reasoning as its existing demo-pay/refund routes).
type Client struct {
	httpClient  *http.Client
	baseURL     string
	internalKey string
}

func NewClient(baseURL, internalKey string) *Client {
	return &Client{httpClient: &http.Client{Timeout: 10 * time.Second}, baseURL: baseURL, internalKey: internalKey}
}

type refundRequest struct {
	PaymentID string `json:"payment_id"`
	// Centavos to give back; omitted = the whole payment.
	AmountCentavos *int `json:"amount_centavos,omitempty"`
}

func (client *Client) RefundPayment(ctx context.Context, paymentID string) error {
	return client.refund(ctx, paymentID, nil)
}

// RefundPaymentAmount gives back part of a payment (an admin's partial refund), in centavos.
func (client *Client) RefundPaymentAmount(ctx context.Context, paymentID string, amountCentavos int) error {
	return client.refund(ctx, paymentID, &amountCentavos)
}

func (client *Client) refund(ctx context.Context, paymentID string, amountCentavos *int) error {
	// A cash booking never had a real charge -- nothing to refund, and
	// payment-service would just reject an unknown payment_id anyway.
	if paymentID == constants.CashPaymentID {
		return nil
	}

	body, err := json.Marshal(refundRequest{PaymentID: paymentID, AmountCentavos: amountCentavos})
	if err != nil {
		return errors.NewUpstream("paymentclient.RefundPayment", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/internal/refund-by-payment", bytes.NewReader(body))
	if err != nil {
		return errors.NewUpstream("paymentclient.RefundPayment", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-Key", client.internalKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return errors.NewUpstream("paymentclient.RefundPayment", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusAccepted:
		// Mercado Pago failed the refund, but payment-service has it on record as owed: its
		// retry job tries again and the admin panel lists it until someone resolves it. The
		// cancellation that asked for it stands -- not an error for whoever called it off
		// (payment-service logs the failure itself).
		return nil
	default:
		return errors.NewUpstream("paymentclient.RefundPayment", fmt.Errorf("unexpected status %d", response.StatusCode))
	}
}
