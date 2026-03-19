package grpctransport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/barn0w1/hss-science/server/gen/blob/v1"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/app"
	"github.com/barn0w1/hss-science/server/services/blob-service/internal/domain"
)

type Server struct {
	pb.UnimplementedBlobServiceServer
	app *app.App
}

func NewServer(a *app.App) *Server {
	return &Server{app: a}
}

func (s *Server) InitiateUpload(ctx context.Context, req *pb.InitiateUploadRequest) (*pb.InitiateUploadResponse, error) {
	result, err := s.app.InitiateUpload(ctx, domain.BlobID(req.BlobId), req.SizeBytes, req.ContentType)
	if err != nil {
		return nil, mapError(err)
	}

	resp := &pb.InitiateUploadResponse{AlreadyExists: result.AlreadyExists}
	if !result.AlreadyExists {
		resp.PresignedPutUrl = result.PresignedPutURL
		resp.UrlExpiresAt = timestamppb.New(result.ExpiresAt)
	}
	return resp, nil
}

func (s *Server) CompleteUpload(ctx context.Context, req *pb.CompleteUploadRequest) (*pb.CompleteUploadResponse, error) {
	result, err := s.app.CompleteUpload(ctx, domain.BlobID(req.BlobId))
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.CompleteUploadResponse{
		BlobId:      string(result.BlobID),
		CommittedAt: timestamppb.New(result.CommittedAt),
	}, nil
}

func (s *Server) InitiateMultipartUpload(ctx context.Context, req *pb.InitiateMultipartUploadRequest) (*pb.InitiateMultipartUploadResponse, error) {
	result, err := s.app.InitiateMultipartUpload(ctx, domain.BlobID(req.BlobId), req.SizeBytes, req.ContentType, req.PartCount)
	if err != nil {
		return nil, mapError(err)
	}

	if result.AlreadyExists {
		return &pb.InitiateMultipartUploadResponse{AlreadyExists: true}, nil
	}

	parts := make([]*pb.PartUploadURL, len(result.Parts))
	for i, p := range result.Parts {
		parts[i] = &pb.PartUploadURL{
			PartNumber:      p.PartNumber,
			PresignedPutUrl: p.PresignedPutURL,
		}
	}
	return &pb.InitiateMultipartUploadResponse{
		UploadId:     result.UploadID,
		Parts:        parts,
		UrlExpiresAt: timestamppb.New(result.ExpiresAt),
	}, nil
}

func (s *Server) CompleteMultipartUpload(ctx context.Context, req *pb.CompleteMultipartUploadRequest) (*pb.CompleteMultipartUploadResponse, error) {
	parts := make([]domain.CompletedPart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = domain.CompletedPart{PartNumber: p.PartNumber, ETag: p.Etag}
	}

	result, err := s.app.CompleteMultipartUpload(ctx, domain.BlobID(req.BlobId), req.UploadId, parts)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.CompleteMultipartUploadResponse{
		BlobId:      string(result.BlobID),
		CommittedAt: timestamppb.New(result.CommittedAt),
	}, nil
}

func (s *Server) AbortMultipartUpload(ctx context.Context, req *pb.AbortMultipartUploadRequest) (*pb.AbortMultipartUploadResponse, error) {
	if err := s.app.AbortMultipartUpload(ctx, domain.BlobID(req.BlobId), req.UploadId); err != nil {
		return nil, mapError(err)
	}
	return &pb.AbortMultipartUploadResponse{}, nil
}

func (s *Server) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	ttl := time.Duration(req.TtlSeconds) * time.Second
	result, err := s.app.GetDownloadURL(ctx, domain.BlobID(req.BlobId), ttl)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetDownloadURLResponse{
		PresignedGetUrl: result.PresignedGetURL,
		UrlExpiresAt:    timestamppb.New(result.ExpiresAt),
	}, nil
}

func (s *Server) GetBlobInfo(ctx context.Context, req *pb.GetBlobInfoRequest) (*pb.GetBlobInfoResponse, error) {
	blob, err := s.app.GetBlobInfo(ctx, domain.BlobID(req.BlobId))
	if err != nil {
		return nil, mapError(err)
	}

	resp := &pb.GetBlobInfoResponse{
		BlobId:      string(blob.ID),
		SizeBytes:   blob.SizeBytes,
		ContentType: blob.ContentType,
		UploadState: stateToProto(blob.State),
	}
	if blob.CommittedAt != nil {
		resp.CommittedAt = timestamppb.New(*blob.CommittedAt)
	}
	return resp, nil
}

func stateToProto(s domain.UploadState) pb.UploadState {
	switch s {
	case domain.StatePending:
		return pb.UploadState_PENDING
	case domain.StateCommitted:
		return pb.UploadState_COMMITTED
	default:
		return pb.UploadState_UPLOAD_STATE_UNSPECIFIED
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidBlobID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrBlobNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyCommitted):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrBlobPending):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		slog.Error("internal error", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}
