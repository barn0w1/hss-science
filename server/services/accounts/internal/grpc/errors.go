package grpcserver

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

func domainStatus(err error) error {
	switch {
	case errors.Is(err, domerr.ErrNotFound):
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, domerr.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, domerr.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
