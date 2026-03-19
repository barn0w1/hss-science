package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

type BlobRepo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *BlobRepo {
	return &BlobRepo{db: db}
}

type blobRow struct {
	ID          string       `db:"id"`
	SizeBytes   int64        `db:"size_bytes"`
	ContentType string       `db:"content_type"`
	R2Key       string       `db:"r2_key"`
	State       string       `db:"state"`
	CreatedAt   time.Time    `db:"created_at"`
	CommittedAt sql.NullTime `db:"committed_at"`
}

func (r *BlobRepo) FindByID(ctx context.Context, id domain.BlobID) (*domain.Blob, error) {
	var row blobRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, size_bytes, content_type, r2_key, state, created_at, committed_at
		 FROM blobs WHERE id = $1`, string(id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBlobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("blobs.FindByID: %w", err)
	}
	return rowToBlob(row), nil
}

func (r *BlobRepo) Create(ctx context.Context, b *domain.Blob) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO blobs (id, size_bytes, content_type, r2_key, state, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		string(b.ID), b.SizeBytes, b.ContentType, b.R2Key, string(b.State), b.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("blobs.Create: %w", domain.ErrAlreadyCommitted)
		}
		return fmt.Errorf("blobs.Create: %w", err)
	}
	return nil
}

func (r *BlobRepo) MarkCommitted(ctx context.Context, id domain.BlobID, at time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE blobs SET state = 'COMMITTED', committed_at = $2
		 WHERE id = $1 AND state = 'PENDING'`,
		string(id), at,
	)
	if err != nil {
		return fmt.Errorf("blobs.MarkCommitted: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("blobs.MarkCommitted rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("blobs.MarkCommitted: %w", domain.ErrAlreadyCommitted)
	}
	return nil
}

func rowToBlob(row blobRow) *domain.Blob {
	b := &domain.Blob{
		ID:          domain.BlobID(row.ID),
		SizeBytes:   row.SizeBytes,
		ContentType: row.ContentType,
		R2Key:       row.R2Key,
		State:       domain.UploadState(row.State),
		CreatedAt:   row.CreatedAt,
	}
	if row.CommittedAt.Valid {
		t := row.CommittedAt.Time
		b.CommittedAt = &t
	}
	return b
}
