package oidc

import (
	"context"
	"time"
)

var _ DeviceSessionService = (*deviceSessionService)(nil)

type deviceSessionService struct {
	repo DeviceSessionRepository
}

func NewDeviceSessionService(repo DeviceSessionRepository) DeviceSessionService {
	return &deviceSessionService{repo: repo}
}

func (s *deviceSessionService) FindOrCreate(ctx context.Context, id, userID, userAgent, ipAddress, deviceName string) (*DeviceSession, error) {
	return s.repo.FindOrCreate(ctx, id, userID, userAgent, ipAddress, deviceName)
}

func (s *deviceSessionService) RevokeByID(ctx context.Context, id, userID string) error {
	return s.repo.RevokeByID(ctx, id, userID)
}

func (s *deviceSessionService) ListActiveByUserID(ctx context.Context, userID string) ([]*DeviceSession, error) {
	return s.repo.ListActiveByUserID(ctx, userID)
}

func (s *deviceSessionService) DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error) {
	return s.repo.DeleteRevokedBefore(ctx, before)
}
