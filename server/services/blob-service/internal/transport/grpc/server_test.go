package grpctransport_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/barn0w1/hss-science/server/gen/blob/v1"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/app"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
	grpctransport "github.com/barn0w1/hss-science/server/services/blob-service/internal/transport/grpc"
)

const (
	bufSize = 1024 * 1024
	validID = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type mockRepo struct {
	blobs map[domain.BlobID]*domain.Blob
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
	cp := *b
	m.blobs[b.ID] = &cp
	return nil
}

func (m *mockRepo) MarkCommitted(_ context.Context, id domain.BlobID, at time.Time) error {
	b, ok := m.blobs[id]
	if !ok {
		return domain.ErrBlobNotFound
	}
	b.State = domain.StateCommitted
	b.CommittedAt = &at
	return nil
}

type mockStorage struct {
	putURL   string
	getURL   string
	uploadID string
	partURL  string
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
	return nil
}

func (m *mockStorage) AbortMultipartUpload(_ context.Context, _, _ string) error {
	return nil
}

func setupServer(t *testing.T, repo domain.BlobRepository, storage domain.ObjectStorage) pb.BlobServiceClient {
	t.Helper()

	blobApp := app.New(repo, storage, app.Config{
		PresignPutTTL:    15 * time.Minute,
		PresignGetMaxTTL: time.Hour,
	})

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterBlobServiceServer(srv, grpctransport.NewServer(blobApp))

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient("passthrough://bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewBlobServiceClient(conn)
}

func TestServer_InitiateUpload_NewBlob(t *testing.T) {
	storage := &mockStorage{putURL: "https://r2.example.com/put"}
	client := setupServer(t, newMockRepo(), storage)

	resp, err := client.InitiateUpload(context.Background(), &pb.InitiateUploadRequest{
		BlobId:      validID,
		SizeBytes:   1024,
		ContentType: "image/png",
	})
	require.NoError(t, err)
	assert.False(t, resp.AlreadyExists)
	assert.Equal(t, "https://r2.example.com/put", resp.PresignedPutUrl)
}

func TestServer_InitiateUpload_InvalidID(t *testing.T) {
	client := setupServer(t, newMockRepo(), &mockStorage{})

	_, err := client.InitiateUpload(context.Background(), &pb.InitiateUploadRequest{
		BlobId: "invalid",
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestServer_GetBlobInfo_NotFound(t *testing.T) {
	client := setupServer(t, newMockRepo(), &mockStorage{})

	_, err := client.GetBlobInfo(context.Background(), &pb.GetBlobInfoRequest{BlobId: validID})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestServer_GetDownloadURL_Pending(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	client := setupServer(t, repo, &mockStorage{getURL: "https://r2.example.com/get"})

	_, err := client.GetDownloadURL(context.Background(), &pb.GetDownloadURLRequest{
		BlobId:     validID,
		TtlSeconds: 3600,
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestServer_GetDownloadURL_Committed(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	client := setupServer(t, repo, &mockStorage{getURL: "https://r2.example.com/get"})

	resp, err := client.GetDownloadURL(context.Background(), &pb.GetDownloadURLRequest{
		BlobId:     validID,
		TtlSeconds: 3600,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://r2.example.com/get", resp.PresignedGetUrl)
}

func TestServer_CompleteUpload_HappyPath(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 1024, "image/png", time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	client := setupServer(t, repo, &mockStorage{})

	resp, err := client.CompleteUpload(context.Background(), &pb.CompleteUploadRequest{BlobId: validID})
	require.NoError(t, err)
	assert.Equal(t, validID, resp.BlobId)
	assert.NotNil(t, resp.CommittedAt)
}

func TestServer_InitiateMultipartUpload(t *testing.T) {
	storage := &mockStorage{uploadID: "mpu-1", partURL: "https://r2.example.com/part"}
	client := setupServer(t, newMockRepo(), storage)

	resp, err := client.InitiateMultipartUpload(context.Background(), &pb.InitiateMultipartUploadRequest{
		BlobId:      validID,
		SizeBytes:   50 * 1024 * 1024,
		ContentType: "video/mp4",
		PartCount:   3,
	})
	require.NoError(t, err)
	assert.Equal(t, "mpu-1", resp.UploadId)
	assert.Len(t, resp.Parts, 3)
}

func TestServer_AbortMultipartUpload(t *testing.T) {
	client := setupServer(t, newMockRepo(), &mockStorage{})

	_, err := client.AbortMultipartUpload(context.Background(), &pb.AbortMultipartUploadRequest{
		BlobId:   validID,
		UploadId: "mpu-1",
	})
	require.NoError(t, err)
}

func TestServer_GetBlobInfo_Committed(t *testing.T) {
	repo := newMockRepo()
	blob, _ := domain.NewBlob(domain.BlobID(validID), 2048, "application/pdf", time.Now())
	_ = blob.Commit(time.Now())
	repo.blobs[domain.BlobID(validID)] = blob

	client := setupServer(t, repo, &mockStorage{})

	resp, err := client.GetBlobInfo(context.Background(), &pb.GetBlobInfoRequest{BlobId: validID})
	require.NoError(t, err)
	assert.Equal(t, validID, resp.BlobId)
	assert.Equal(t, int64(2048), resp.SizeBytes)
	assert.Equal(t, pb.UploadState_COMMITTED, resp.UploadState)
	assert.NotNil(t, resp.CommittedAt)
}
