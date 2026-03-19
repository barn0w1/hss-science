package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/app"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

const validID = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

var testCfg = app.Config{
	PresignPutTTL:           15 * time.Minute,
	PresignGetMaxTTL:        time.Hour,
	MultipartThresholdBytes: 10 * 1024 * 1024,
}

func newApp(repo domain.BlobRepository, storage domain.ObjectStorage) *app.App {
	return app.New(repo, storage, testCfg)
}

func TestInitiateUpload_NewBlob(t *testing.T) {
	repo := newMockRepo()
	storage := &mockStorage{putURL: "https://r2.example.com/put"}
	a := newApp(repo, storage)

	result, err := a.InitiateUpload(context.Background(), domain.BlobID(validID), 1024, "image/png")
	require.NoError(t, err)
	assert.False(t, result.AlreadyExists)
	assert.Equal(t, "https://r2.example.com/put", result.PresignedPutURL)

	_, err = repo.FindByID(context.Background(), domain.BlobID(validID))
	require.NoError(t, err)
}

func TestInitiateUpload_AlreadyCommitted(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})

	result, err := a.InitiateUpload(context.Background(), domain.BlobID(validID), 1024, "image/png")
	require.NoError(t, err)
	assert.True(t, result.AlreadyExists)
	assert.Empty(t, result.PresignedPutURL)
}

func TestInitiateUpload_Pending_ReissuesURL(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	storage := &mockStorage{putURL: "https://r2.example.com/put2"}
	a := newApp(repo, storage)

	result, err := a.InitiateUpload(context.Background(), domain.BlobID(validID), 1024, "image/png")
	require.NoError(t, err)
	assert.False(t, result.AlreadyExists)
	assert.Equal(t, "https://r2.example.com/put2", result.PresignedPutURL)
}

func TestInitiateUpload_InvalidID(t *testing.T) {
	a := newApp(newMockRepo(), &mockStorage{})
	_, err := a.InitiateUpload(context.Background(), "invalid", 1024, "image/png")
	assert.ErrorIs(t, err, domain.ErrInvalidBlobID)
}

func TestCompleteUpload_HappyPath(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	result, err := a.CompleteUpload(context.Background(), domain.BlobID(validID))
	require.NoError(t, err)
	assert.Equal(t, domain.BlobID(validID), result.BlobID)
	assert.False(t, result.CommittedAt.IsZero())

	updated, _ := repo.FindByID(context.Background(), domain.BlobID(validID))
	assert.Equal(t, domain.StateCommitted, updated.State)
}

func TestCompleteUpload_NotFound(t *testing.T) {
	a := newApp(newMockRepo(), &mockStorage{})
	_, err := a.CompleteUpload(context.Background(), domain.BlobID(validID))
	assert.ErrorIs(t, err, domain.ErrBlobNotFound)
}

func TestCompleteUpload_AlreadyCommitted(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	_, err := a.CompleteUpload(context.Background(), domain.BlobID(validID))
	assert.ErrorIs(t, err, domain.ErrAlreadyCommitted)
}

func TestInitiateMultipartUpload_NewBlob(t *testing.T) {
	repo := newMockRepo()
	storage := &mockStorage{uploadID: "mpu-123", partURL: "https://r2.example.com/part"}
	a := newApp(repo, storage)

	result, err := a.InitiateMultipartUpload(context.Background(), domain.BlobID(validID), 1024*1024*50, "video/mp4", 3)
	require.NoError(t, err)
	assert.False(t, result.AlreadyExists)
	assert.Equal(t, "mpu-123", result.UploadID)
	assert.Len(t, result.Parts, 3)
	for i, p := range result.Parts {
		assert.Equal(t, int32(i+1), p.PartNumber)
		assert.Equal(t, "https://r2.example.com/part", p.PresignedPutURL)
	}
}

func TestInitiateMultipartUpload_AlreadyCommitted(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	result, err := a.InitiateMultipartUpload(context.Background(), domain.BlobID(validID), 1024, "image/png", 1)
	require.NoError(t, err)
	assert.True(t, result.AlreadyExists)
}

func TestInitiateMultipartUpload_ZeroPartCount(t *testing.T) {
	a := newApp(newMockRepo(), &mockStorage{})
	_, err := a.InitiateMultipartUpload(context.Background(), domain.BlobID(validID), 1024, "image/png", 0)
	assert.Error(t, err)
}

func TestCompleteMultipartUpload_HappyPath(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	parts := []domain.CompletedPart{{PartNumber: 1, ETag: `"abc123"`}}
	result, err := a.CompleteMultipartUpload(context.Background(), domain.BlobID(validID), "mpu-123", parts)
	require.NoError(t, err)
	assert.Equal(t, domain.BlobID(validID), result.BlobID)

	updated, _ := repo.FindByID(context.Background(), domain.BlobID(validID))
	assert.Equal(t, domain.StateCommitted, updated.State)
}

func TestGetDownloadURL_Committed(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	storage := &mockStorage{getURL: "https://r2.example.com/download"}
	a := newApp(repo, storage)

	result, err := a.GetDownloadURL(context.Background(), domain.BlobID(validID), 30*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://r2.example.com/download", result.PresignedGetURL)
}

func TestGetDownloadURL_Pending(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	_, err := a.GetDownloadURL(context.Background(), domain.BlobID(validID), 30*time.Minute)
	assert.ErrorIs(t, err, domain.ErrBlobPending)
}

func TestGetDownloadURL_TTLCeiling(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	storage := &mockStorage{getURL: "https://r2.example.com/download"}
	cfg := app.Config{PresignPutTTL: 15 * time.Minute, PresignGetMaxTTL: time.Hour}
	a := app.New(repo, storage, cfg)

	before := time.Now()
	result, err := a.GetDownloadURL(context.Background(), domain.BlobID(validID), 24*time.Hour)
	require.NoError(t, err)
	assert.True(t, result.ExpiresAt.Before(before.Add(time.Hour+time.Second)), "TTL should be capped at 1h")
}

func TestGetBlobInfo(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 2048, "application/pdf", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	a := newApp(repo, &mockStorage{})
	info, err := a.GetBlobInfo(context.Background(), domain.BlobID(validID))
	require.NoError(t, err)
	assert.Equal(t, domain.BlobID(validID), info.ID)
	assert.Equal(t, int64(2048), info.SizeBytes)
	assert.Equal(t, domain.StatePending, info.State)
}
