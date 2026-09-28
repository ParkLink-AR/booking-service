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

// Client uploads objects to Cloudflare R2 (S3-compatible) -- settlement proof images live
// under `payout-proofs/{ownerId}/{settlementId}.{ext}` (see usecase.ConfirmSettlementUseCase),
// one folder per owner so every proof for a given owner sits together. Mirrors how
// parking-service keys photos under `parking-spots/{spotId}/`.
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

// UploadPayoutProof uploads a bank-transfer proof image to ownerID's folder, named after key
// (the settlement's own id -- see usecase.ConfirmSettlementUseCase, which is the only caller
// left of this method now that payouts settle in batches), and returns its public URL. A
// failed upload aborts the whole confirm call, so a settlement is never confirmed PAID
// without whatever proof the admin attached. Re-uploading a proof for the same settlement
// overwrites the previous object at that key -- acceptable since a settlement can only be
// confirmed once (see ConfirmSettlement's PENDING-only check), so this path is never written
// twice for the same settlement.
func (client *Client) UploadPayoutProof(ctx context.Context, ownerID, key, contentType string, data []byte) (string, error) {
	objectKey := fmt.Sprintf("payout-proofs/%s/%s.%s", ownerID, key, extensionFor(contentType))

	if _, err := client.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(client.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", errors.NewInternalError("storage.UploadPayoutProof", err)
	}

	return fmt.Sprintf("%s/%s", client.publicURL, objectKey), nil
}
