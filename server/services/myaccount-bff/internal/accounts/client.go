package accounts

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

type Client struct {
	svc pb.AccountManagementServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}
	return &Client{svc: pb.NewAccountManagementServiceClient(conn)}, nil
}

func (c *Client) withBearer(ctx context.Context, token string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}

func (c *Client) GetMyProfile(ctx context.Context, token string) (*pb.Profile, error) {
	return c.svc.GetMyProfile(c.withBearer(ctx, token), &pb.GetMyProfileRequest{})
}

func (c *Client) UpdateMyProfile(ctx context.Context, token string, name, picture *string) (*pb.Profile, error) {
	req := &pb.UpdateMyProfileRequest{
		Name:    name,
		Picture: picture,
	}
	return c.svc.UpdateMyProfile(c.withBearer(ctx, token), req)
}

func (c *Client) ListLinkedProviders(ctx context.Context, token string) ([]*pb.FederatedProviderInfo, error) {
	resp, err := c.svc.ListLinkedProviders(c.withBearer(ctx, token), &pb.ListLinkedProvidersRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetProviders(), nil
}

func (c *Client) UnlinkProvider(ctx context.Context, token, identityID string) error {
	_, err := c.svc.UnlinkProvider(c.withBearer(ctx, token), &pb.UnlinkProviderRequest{IdentityId: identityID})
	return err
}

func (c *Client) ListActiveSessions(ctx context.Context, token string) ([]*pb.Session, error) {
	resp, err := c.svc.ListActiveSessions(c.withBearer(ctx, token), &pb.ListActiveSessionsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetSessions(), nil
}

func (c *Client) RevokeSession(ctx context.Context, token, sessionID string) error {
	_, err := c.svc.RevokeSession(c.withBearer(ctx, token), &pb.RevokeSessionRequest{SessionId: sessionID})
	return err
}

func (c *Client) RevokeAllOtherSessions(ctx context.Context, token, currentSessionID string) error {
	_, err := c.svc.RevokeAllOtherSessions(c.withBearer(ctx, token), &pb.RevokeAllOtherSessionsRequest{CurrentSessionId: currentSessionID})
	return err
}
