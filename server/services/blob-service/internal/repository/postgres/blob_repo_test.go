package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/repository/postgres"
	"github.com/barn0w1/hss-science/server/services/blob-service/testhelper"
)

const validID = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestCreate_and_FindByID(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	blob, err := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, err)

	require.NoError(t, repo.Create(context.Background(), blob))

	got, err := repo.FindByID(context.Background(), domain.BlobID(validID))
	require.NoError(t, err)
	assert.Equal(t, blob.ID, got.ID)
	assert.Equal(t, blob.SizeBytes, got.SizeBytes)
	assert.Equal(t, blob.ContentType, got.ContentType)
	assert.Equal(t, blob.R2Key, got.R2Key)
	assert.Equal(t, domain.StatePending, got.State)
	assert.Nil(t, got.CommittedAt)
}

func TestFindByID_NotFound(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	_, err := repo.FindByID(context.Background(), domain.BlobID(validID))
	assert.ErrorIs(t, err, domain.ErrBlobNotFound)
}

func TestMarkCommitted(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now().UTC())
	require.NoError(t, repo.Create(context.Background(), blob))

	at := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.MarkCommitted(context.Background(), domain.BlobID(validID), at))

	got, err := repo.FindByID(context.Background(), domain.BlobID(validID))
	require.NoError(t, err)
	assert.Equal(t, domain.StateCommitted, got.State)
	require.NotNil(t, got.CommittedAt)
	assert.Equal(t, at, got.CommittedAt.UTC().Truncate(time.Microsecond))
}

func TestMarkCommitted_AlreadyCommitted(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now().UTC())
	require.NoError(t, repo.Create(context.Background(), blob))
	require.NoError(t, repo.MarkCommitted(context.Background(), domain.BlobID(validID), time.Now()))

	err := repo.MarkCommitted(context.Background(), domain.BlobID(validID), time.Now())
	assert.ErrorIs(t, err, domain.ErrAlreadyCommitted)
}

func TestMarkCommitted_NotFound(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	err := repo.MarkCommitted(context.Background(), domain.BlobID(validID), time.Now())
	assert.ErrorIs(t, err, domain.ErrAlreadyCommitted)
}

func TestCreate_DuplicateKey(t *testing.T) {
	db := testhelper.NewTestDB(t)
	repo := postgres.New(db)

	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now().UTC())
	require.NoError(t, repo.Create(context.Background(), blob))

	err := repo.Create(context.Background(), blob)
	assert.Error(t, err)
}
