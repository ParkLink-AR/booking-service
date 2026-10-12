// Package storage uploads payout proof images to their own private Cloudflare R2 bucket --
// separate from the public bucket auth-service (app/utils/storage.py) and parking-service
// (src/storage/storage.service.ts) write avatars/spot photos to. A bank-transfer receipt is a
// private financial document, not something that belongs on a publicly reachable, guessable
// URL the way a spot photo does, so this bucket has no public custom domain attached: reads go
// through ResolveProofURL's short-lived presigned URLs instead (see proof_url.go in
// internal/usecase/booking).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/example/bookings-service/internal/domain/errors"
)

// proofURLTTL is how long a presigned proof URL stays valid -- long enough for an admin or
// owner to open the settlement detail screen and have the image load, short enough that a
// leaked link (e.g. pasted into a chat) doesn't stay usable indefinitely. Every read re-signs
// a fresh one (see ResolveProofURL), so this never limits how long after confirmation a proof
// stays viewable -- only how long any single generated link lives.
const proofURLTTL = 1 * time.Hour

// Client uploads objects to Cloudflare R2 (S3-compatible) -- settlement proof images live
// under `payout-proofs/{ownerId}/{settlementId}.{ext}` (see usecase.ConfirmSettlementUseCase),
// one folder per owner so every proof for a given owner sits together. Mirrors how
// parking-service keys photos under `parking-spots/{spotId}/`.
type Client struct {
	s3Client *s3.Client
	bucket   string
}

// NewClient builds an R2-backed client for the private payout-proofs bucket (accountID is the
// same Cloudflare account the public photos bucket lives in, but bucket/accessKeyID/
// secretAccessKey must point at the private one -- see R2_PAYOUT_PROOFS_BUCKET_NAME).
func NewClient(ctx context.Context, accountID, bucket, accessKeyID, secretAccessKey string) (*Client, error) {
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

	return &Client{s3Client: s3Client, bucket: bucket}, nil
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
// left of this method now that payouts settle in batches), and returns its object key --
// never a URL, since the bucket is private. ResolveProofURL turns that key into something
// loadable, generated fresh on every read rather than baked in once here, since a key never
// expires but a signed URL does. A failed upload aborts the whole confirm call, so a
// settlement is never confirmed PAID without whatever proof the admin attached. Re-uploading a
// proof for the same settlement overwrites the previous object at that key -- acceptable since
// a settlement can only be confirmed once (see ConfirmSettlement's PENDING-only check), so this
// path is never written twice for the same settlement.
//
// An image proof is compressed first (see compressImage); anything else is stored as sent.
func (client *Client) UploadPayoutProof(ctx context.Context, ownerID, key, contentType string, data []byte) (string, error) {
	if compressed, ok := compressImage(data); ok {
		data, contentType = compressed, "image/jpeg"
	}
	objectKey := fmt.Sprintf("payout-proofs/%s/%s.%s", ownerID, key, extensionFor(contentType))

	if _, err := client.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(client.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", errors.NewInternalError("storage.UploadPayoutProof", err)
	}

	return objectKey, nil
}

// ResolveProofURL turns a stored proof reference into a URL the caller can actually load. A
// bare object key (anything UploadPayoutProof has ever returned) gets a fresh presigned GET
// URL, valid for proofURLTTL -- cheap and purely local (HMAC signing, no network round-trip),
// so there's no real failure mode in practice beyond a malformed key. A value that's already a
// full "http(s)://..." URL predates this private-bucket migration (back when proofs lived in
// the same public bucket as spot photos, see git history) and is passed through unchanged:
// that object still only exists in the old public bucket, so there's nothing of ours to sign.
func (client *Client) ResolveProofURL(ctx context.Context, keyOrURL string) (string, error) {
	if strings.HasPrefix(keyOrURL, "http://") || strings.HasPrefix(keyOrURL, "https://") {
		return keyOrURL, nil
	}

	presignClient := s3.NewPresignClient(client.s3Client)
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(keyOrURL),
	}, s3.WithPresignExpires(proofURLTTL))
	if err != nil {
		return "", errors.NewInternalError("storage.ResolveProofURL", err)
	}
	return request.URL, nil
}
