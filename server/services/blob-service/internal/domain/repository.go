package domain

import (
	"context"
	"time"
)

type BlobRepository interface {
	FindByID(ctx context.Context, id BlobID) (*Blob, error)
	Create(ctx context.Context, b *Blob) error
	MarkCommitted(ctx context.Context, id BlobID, at time.Time) error
}
