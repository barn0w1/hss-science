package grpcserver

import (
	"google.golang.org/grpc"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	oidcadapter "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc/adapter"
)

func NewServer(
	identitySvc identity.Service,
	deviceSessionSvc oidcdom.DeviceSessionService,
	publicKeys *oidcadapter.PublicKeySet,
	issuer string,
) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			NewJWTAuthInterceptor(publicKeys, issuer),
		),
	)
	pb.RegisterAccountManagementServiceServer(srv, &Handler{
		identitySvc:      identitySvc,
		deviceSessionSvc: deviceSessionSvc,
	})
	return srv
}
