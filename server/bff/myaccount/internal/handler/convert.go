package handler

import (
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	myaccountv1 "github.com/barn0w1/hss-science/server/bff/gen/myaccount/v1"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// protoProfileToAPI converts a gRPC Profile to the generated OpenAPI Profile type.
func protoProfileToAPI(p *pb.Profile) myaccountv1.Profile {
	userID, _ := uuid.Parse(p.UserId)

	var picture *string
	if p.Picture != "" {
		picture = &p.Picture
	}

	return myaccountv1.Profile{
		UserId:        userID,
		Email:         openapi_types.Email(p.Email),
		EmailVerified: p.EmailVerified,
		GivenName:     p.GivenName,
		FamilyName:    p.FamilyName,
		Picture:       picture,
		Locale:        p.Locale,
		CreatedAt:     p.CreatedAt.AsTime(),
		UpdatedAt:     p.UpdatedAt.AsTime(),
	}
}

// protoLinkedAccountToAPI converts a gRPC LinkedAccount to the generated OpenAPI LinkedAccount type.
func protoLinkedAccountToAPI(la *pb.LinkedAccount) myaccountv1.LinkedAccount {
	id, _ := uuid.Parse(la.Id)
	return myaccountv1.LinkedAccount{
		Id:          id,
		Provider:    la.Provider,
		ExternalSub: la.ExternalSub,
		LinkedAt:    la.LinkedAt.AsTime(),
	}
}

// protoSessionToAPI converts a gRPC Session to the generated OpenAPI Session type.
func protoSessionToAPI(s *pb.Session) myaccountv1.Session {
	sid, _ := uuid.Parse(s.SessionId)
	return myaccountv1.Session{
		SessionId: sid,
		ClientId:  s.ClientId,
		Scopes:    s.Scopes,
		AuthTime:  s.AuthTime.AsTime(),
		ExpiresAt: s.ExpiresAt.AsTime(),
		CreatedAt: s.CreatedAt.AsTime(),
	}
}
