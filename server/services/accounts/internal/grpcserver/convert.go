package grpcserver

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
)

func userToProto(u *storage.User) *pb.Profile {
	return &pb.Profile{
		UserId:        u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		GivenName:     u.GivenName,
		FamilyName:    u.FamilyName,
		Picture:       u.Picture,
		Locale:        u.Locale,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
	}
}

func linkedAccountToProto(fi *storage.FederatedIdentity) *pb.LinkedAccount {
	return &pb.LinkedAccount{
		Id:          fi.ID,
		Provider:    fi.Provider,
		ExternalSub: fi.ExternalSub,
		LinkedAt:    timestamppb.New(fi.CreatedAt),
	}
}

func sessionToProto(rt *storage.RefreshToken) *pb.Session {
	return &pb.Session{
		SessionId: rt.ID,
		ClientId:  rt.ClientID,
		Scopes:    []string(rt.Scopes),
		AuthTime:  timestamppb.New(rt.AuthTime),
		ExpiresAt: timestamppb.New(rt.ExpiresAt),
		CreatedAt: timestamppb.New(rt.CreatedAt),
	}
}
