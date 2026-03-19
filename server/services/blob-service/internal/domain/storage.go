package domain

import (
	"context"
	"time"
)

type CompletedPart struct {
	PartNumber int32
	ETag       string
}

type ObjectStorage interface {
	PresignedPutURL(ctx context.Context, key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
	PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
	CreateMultipartUpload(ctx context.Context, key, contentType string) (uploadID string, err error)
	PresignedPartURL(ctx context.Context, key, uploadID string, partNumber int32, ttl time.Duration) (url string, expiresAt time.Time, err error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}
