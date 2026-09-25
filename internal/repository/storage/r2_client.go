// Package storage uploads payout proof images to the same Cloudflare R2 bucket that
// parking-service (src/storage/storage.service.ts) and auth-service (app/utils/storage.py)
// already write to, via R2's S3-compatible API.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/example/bookings-service/internal/domain/errors"
)

// Client uploads objects to Cloudflare R2 (S3-compatible) -- payout proof images live under
// `payout-proofs/{ownerId}/{paymentId}.{ext}`, one folder per owner (so every proof for a
// given owner sits together) named by the booking's payment reference (so a proof is
// identifiable without joining back to bookings/payouts). Mirrors how parking-service keys
// photos under `parking-spots/{spotId}/`.
type Client struct {
	s3Client  *s3.Client
	bucket    string
	publicURL string
}

// NewClient builds an R2-backed client from the same env vars the other two services read
// (R2_ACCOUNT_ID, R2_BUCKET_NAME, R2_PUBLIC_URL, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY), for
// ops consistency.
func NewClient(ctx context.Context, accountID, bucket, publicURL, accessKeyID, secretAccessKey string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, errors.NewInternalError("storage.NewClient", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID))
	})

	return &Client{
		s3Client:  s3Client,
		bucket:    bucket,
		publicURL: strings.TrimRight(publicURL, "/"),
	}, nil
}

// extensionFor maps a content type to a file extension, mirroring parking-service's
// extensionFor (src/parking-spots/parking-spots.controller.ts) -- "image/jpeg" -> "jpg".
func extensionFor(contentType string) string {
	_, subtype, ok := strings.Cut(contentType, "/")
	if !ok || subtype == "" {
		return "bin"
	}
	if subtype == "jpeg" {
		return "jpg"
	}
	return subtype
}

// UploadPayoutProof uploads a bank-transfer proof image for a payout to ownerID's folder,
// named after paymentID (the booking's Mercado Pago payment reference), and returns its
// public URL. Used by MarkPayoutPaidUseCase -- a failed upload aborts the whole mark-paid
// call, so the payout is never marked PAID without whatever proof the admin attached.
// Re-uploading a proof for the same payment reference overwrites the previous object at
// that key -- acceptable since a payout can only be marked paid once (see MarkPayoutPaid's
// PENDING-only WHERE clause), so this path is never written twice for the same payout.
func (client *Client) UploadPayoutProof(ctx context.Context, ownerID, paymentID, contentType string, data []byte) (string, error) {
	key := fmt.Sprintf("payout-proofs/%s/%s.%s", ownerID, paymentID, extensionFor(contentType))

	if _, err := client.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(client.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", errors.NewInternalError("storage.UploadPayoutProof", err)
	}

	return fmt.Sprintf("%s/%s", client.publicURL, key), nil
}
