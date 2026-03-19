package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

type Config struct {
	PresignPutTTL           time.Duration
	PresignGetMaxTTL        time.Duration
	MultipartThresholdBytes int64
}

type App struct {
	repo    domain.BlobRepository
	storage domain.ObjectStorage
	cfg     Config
}

func New(repo domain.BlobRepository, storage domain.ObjectStorage, cfg Config) *App {
	return &App{repo: repo, storage: storage, cfg: cfg}
}

type InitiateUploadResult struct {
	AlreadyExists   bool
	PresignedPutURL string
	ExpiresAt       time.Time
}

func (a *App) InitiateUpload(ctx context.Context, id domain.BlobID, sizeBytes int64, contentType string) (*InitiateUploadResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	existing, err := a.repo.FindByID(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrBlobNotFound) {
		return nil, fmt.Errorf("InitiateUpload: %w", err)
	}

	if existing != nil && existing.State == domain.StateCommitted {
		return &InitiateUploadResult{AlreadyExists: true}, nil
	}

	if existing == nil {
		blob, err := domain.NewBlob(id, sizeBytes, contentType, time.Now().UTC())
		if err != nil {
			return nil, fmt.Errorf("InitiateUpload: %w", err)
		}
		if err := a.repo.Create(ctx, blob); err != nil {
			return nil, fmt.Errorf("InitiateUpload: %w", err)
		}
	}

	url, expiresAt, err := a.storage.PresignedPutURL(ctx, string(id), a.cfg.PresignPutTTL)
	if err != nil {
		return nil, fmt.Errorf("InitiateUpload: %w", err)
	}
	return &InitiateUploadResult{PresignedPutURL: url, ExpiresAt: expiresAt}, nil
}

type CompleteUploadResult struct {
	BlobID      domain.BlobID
	CommittedAt time.Time
}

func (a *App) CompleteUpload(ctx context.Context, id domain.BlobID) (*CompleteUploadResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	blob, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("CompleteUpload: %w", err)
	}
	if blob.State == domain.StateCommitted {
		return nil, fmt.Errorf("CompleteUpload: %w", domain.ErrAlreadyCommitted)
	}

	now := time.Now().UTC()
	if err := a.repo.MarkCommitted(ctx, id, now); err != nil {
		return nil, fmt.Errorf("CompleteUpload: %w", err)
	}
	return &CompleteUploadResult{BlobID: id, CommittedAt: now}, nil
}

type InitiateMultipartResult struct {
	AlreadyExists bool
	UploadID      string
	Parts         []PartUploadURL
	ExpiresAt     time.Time
}

type PartUploadURL struct {
	PartNumber      int32
	PresignedPutURL string
}

func (a *App) InitiateMultipartUpload(ctx context.Context, id domain.BlobID, sizeBytes int64, contentType string, partCount int32) (*InitiateMultipartResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	if partCount < 1 {
		return nil, fmt.Errorf("part_count must be at least 1")
	}

	existing, err := a.repo.FindByID(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrBlobNotFound) {
		return nil, fmt.Errorf("InitiateMultipartUpload: %w", err)
	}

	if existing != nil && existing.State == domain.StateCommitted {
		return &InitiateMultipartResult{AlreadyExists: true}, nil
	}

	if existing == nil {
		blob, err := domain.NewBlob(id, sizeBytes, contentType, time.Now().UTC())
		if err != nil {
			return nil, fmt.Errorf("InitiateMultipartUpload: %w", err)
		}
		if err := a.repo.Create(ctx, blob); err != nil {
			return nil, fmt.Errorf("InitiateMultipartUpload: %w", err)
		}
	}

	uploadID, err := a.storage.CreateMultipartUpload(ctx, string(id), contentType)
	if err != nil {
		return nil, fmt.Errorf("InitiateMultipartUpload: %w", err)
	}

	parts := make([]PartUploadURL, partCount)
	var expiresAt time.Time
	for i := int32(0); i < partCount; i++ {
		partNum := i + 1
		url, exp, err := a.storage.PresignedPartURL(ctx, string(id), uploadID, partNum, a.cfg.PresignPutTTL)
		if err != nil {
			return nil, fmt.Errorf("InitiateMultipartUpload part %d: %w", partNum, err)
		}
		parts[i] = PartUploadURL{PartNumber: partNum, PresignedPutURL: url}
		expiresAt = exp
	}

	return &InitiateMultipartResult{
		UploadID:  uploadID,
		Parts:     parts,
		ExpiresAt: expiresAt,
	}, nil
}

type CompleteMultipartResult struct {
	BlobID      domain.BlobID
	CommittedAt time.Time
}

func (a *App) CompleteMultipartUpload(ctx context.Context, id domain.BlobID, uploadID string, parts []domain.CompletedPart) (*CompleteMultipartResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	blob, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("CompleteMultipartUpload: %w", err)
	}
	if blob.State == domain.StateCommitted {
		return nil, fmt.Errorf("CompleteMultipartUpload: %w", domain.ErrAlreadyCommitted)
	}

	if err := a.storage.CompleteMultipartUpload(ctx, string(id), uploadID, parts); err != nil {
		return nil, fmt.Errorf("CompleteMultipartUpload: %w", err)
	}

	now := time.Now().UTC()
	if err := a.repo.MarkCommitted(ctx, id, now); err != nil {
		return nil, fmt.Errorf("CompleteMultipartUpload: %w", err)
	}
	return &CompleteMultipartResult{BlobID: id, CommittedAt: now}, nil
}

func (a *App) AbortMultipartUpload(ctx context.Context, id domain.BlobID, uploadID string) error {
	if err := id.Validate(); err != nil {
		return err
	}
	if err := a.storage.AbortMultipartUpload(ctx, string(id), uploadID); err != nil {
		return fmt.Errorf("AbortMultipartUpload: %w", err)
	}
	return nil
}

type GetDownloadURLResult struct {
	PresignedGetURL string
	ExpiresAt       time.Time
}

func (a *App) GetDownloadURL(ctx context.Context, id domain.BlobID, ttl time.Duration) (*GetDownloadURLResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	if ttl <= 0 || ttl > a.cfg.PresignGetMaxTTL {
		ttl = a.cfg.PresignGetMaxTTL
	}

	blob, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetDownloadURL: %w", err)
	}
	if blob.State == domain.StatePending {
		return nil, fmt.Errorf("GetDownloadURL: %w", domain.ErrBlobPending)
	}

	url, expiresAt, err := a.storage.PresignedGetURL(ctx, string(id), ttl)
	if err != nil {
		return nil, fmt.Errorf("GetDownloadURL: %w", err)
	}
	return &GetDownloadURLResult{PresignedGetURL: url, ExpiresAt: expiresAt}, nil
}

func (a *App) GetBlobInfo(ctx context.Context, id domain.BlobID) (*domain.Blob, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	blob, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetBlobInfo: %w", err)
	}
	return blob, nil
}
