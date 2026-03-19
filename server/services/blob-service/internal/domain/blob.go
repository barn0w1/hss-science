package domain

import (
	"fmt"
	"regexp"
	"time"
)

var hexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

type BlobID string

func (id BlobID) Validate() error {
	if !hexRE.MatchString(string(id)) {
		return ErrInvalidBlobID
	}
	return nil
}

type UploadState string

const (
	StatePending   UploadState = "PENDING"
	StateCommitted UploadState = "COMMITTED"
)

type Blob struct {
	ID          BlobID
	SizeBytes   int64
	ContentType string
	R2Key       string
	State       UploadState
	CreatedAt   time.Time
	CommittedAt *time.Time
}

func NewBlob(id BlobID, sizeBytes int64, contentType string, now time.Time) (*Blob, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return &Blob{
		ID:          id,
		SizeBytes:   sizeBytes,
		ContentType: contentType,
		R2Key:       string(id),
		State:       StatePending,
		CreatedAt:   now,
	}, nil
}

func (b *Blob) Commit(at time.Time) error {
	if b.State == StateCommitted {
		return fmt.Errorf("%w: blob %s", ErrAlreadyCommitted, b.ID)
	}
	b.State = StateCommitted
	b.CommittedAt = &at
	return nil
}
