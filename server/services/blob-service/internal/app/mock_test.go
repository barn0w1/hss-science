package app_test

import (
	"context"
	"time"

	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

type mockRepo struct {
	blobs     map[domain.BlobID]*domain.Blob
	createErr error
	commitErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{blobs: make(map[domain.BlobID]*domain.Blob)}
}

func (m *mockRepo) FindByID(_ context.Context, id domain.BlobID) (*domain.Blob, error) {
	b, ok := m.blobs[id]
	if !ok {
		return nil, domain.ErrBlobNotFound
	}
	return b, nil
}

func (m *mockRepo) Create(_ context.Context, b *domain.Blob) error {
	if m.createErr != nil {
		return m.createErr
	}
	cp := *b
	m.blobs[b.ID] = &cp
	return nil
}

func (m *mockRepo) MarkCommitted(_ context.Context, id domain.BlobID, at time.Time) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	b, ok := m.blobs[id]
	if !ok {
		return domain.ErrBlobNotFound
	}
	b.State = domain.StateCommitted
	b.CommittedAt = &at
	return nil
}

type mockStorage struct {
	putURL      string
	getURL      string
	uploadID    string
	partURL     string
	completeErr error
	abortErr    error
}

func (m *mockStorage) PresignedPutURL(_ context.Context, _ string, _ time.Duration) (string, time.Time, error) {
	return m.putURL, time.Now().Add(time.Minute), nil
}

func (m *mockStorage) PresignedGetURL(_ context.Context, _ string, ttl time.Duration) (string, time.Time, error) {
	return m.getURL, time.Now().Add(ttl), nil
}

func (m *mockStorage) CreateMultipartUpload(_ context.Context, _, _ string) (string, error) {
	return m.uploadID, nil
}

func (m *mockStorage) PresignedPartURL(_ context.Context, _, _ string, _ int32, _ time.Duration) (string, time.Time, error) {
	return m.partURL, time.Now().Add(time.Minute), nil
}

func (m *mockStorage) CompleteMultipartUpload(_ context.Context, _, _ string, _ []domain.CompletedPart) error {
	return m.completeErr
}

func (m *mockStorage) AbortMultipartUpload(_ context.Context, _, _ string) error {
	return m.abortErr
}
