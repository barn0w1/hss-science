package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

const validID = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestBlobID_Validate(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid", validID, false},
		{"too short", "e3b0c44298fc1c149afbf4c8996fb924", true},
		{"too long", validID + "aa", true},
		{"uppercase", "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", true},
		{"non-hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", true},
		{"empty", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.BlobID(tc.id).Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewBlob(t *testing.T) {
	now := time.Now().UTC()
	blob, err := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", now)
	require.NoError(t, err)

	assert.Equal(t, domain.BlobID(validID), blob.ID)
	assert.Equal(t, int64(1024), blob.SizeBytes)
	assert.Equal(t, "image/png", blob.ContentType)
	assert.Equal(t, validID, blob.R2Key, "R2Key must equal ID (CAS invariant)")
	assert.Equal(t, domain.StatePending, blob.State)
	assert.Equal(t, now, blob.CreatedAt)
	assert.Nil(t, blob.CommittedAt)
}

func TestNewBlob_InvalidID(t *testing.T) {
	_, err := domain.NewBlob("invalid", 1024, "image/png", time.Now())
	assert.ErrorIs(t, err, domain.ErrInvalidBlobID)
}

func TestBlob_Commit(t *testing.T) {
	blob, err := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	require.NoError(t, err)

	at := time.Now().UTC()
	err = blob.Commit(at)
	require.NoError(t, err)
	assert.Equal(t, domain.StateCommitted, blob.State)
	require.NotNil(t, blob.CommittedAt)
	assert.Equal(t, at, *blob.CommittedAt)
}

func TestBlob_Commit_AlreadyCommitted(t *testing.T) {
	blob, err := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	require.NoError(t, err)

	require.NoError(t, blob.Commit(time.Now()))
	err = blob.Commit(time.Now())
	assert.ErrorIs(t, err, domain.ErrAlreadyCommitted)
}
