package grpcclient

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// Client wraps the gRPC connection to the accounts service.
type Client struct {
	conn    *grpc.ClientConn
	service pb.AccountsServiceClient
}

// NewClient creates a new gRPC client connected to the given address.
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		service: pb.NewAccountsServiceClient(conn),
	}, nil
}

// Service returns the typed gRPC client for the accounts service.
func (c *Client) Service() pb.AccountsServiceClient {
	return c.service
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// WithToken returns a context with the bearer token set in gRPC metadata.
func WithToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}
