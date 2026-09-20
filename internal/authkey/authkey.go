// Package authkey implements a minimal API-key check shared by every
// LifeSure service, standing in for a real auth system (mTLS, OAuth, etc.)
// for training purposes.
package authkey

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const metadataKey = "x-api-key"

// DemoKey is the shared secret every LifeSure client and server in this
// training project is configured with.
const DemoKey = "lifesure-demo-key"

// UnaryServerInterceptor rejects unary calls that don't carry the expected
// API key in their metadata.
func UnaryServerInterceptor(expectedKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := authorize(ctx, expectedKey); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor rejects streaming calls that don't carry the
// expected API key in their metadata.
func StreamServerInterceptor(expectedKey string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := authorize(ss.Context(), expectedKey); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func authorize(ctx context.Context, expectedKey string) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing request metadata")
	}
	values := md.Get(metadataKey)
	if len(values) == 0 || values[0] != expectedKey {
		return status.Error(codes.Unauthenticated, "missing or invalid API key")
	}
	return nil
}

// perRPCCredentials attaches the API key to every outgoing RPC, unary or
// streaming, via credentials.PerRPCCredentials.
type perRPCCredentials struct {
	key string
}

// NewCredentials returns client-side per-RPC credentials that attach key as
// the x-api-key metadata value on every call.
func NewCredentials(key string) credentials.PerRPCCredentials {
	return perRPCCredentials{key: key}
}

func (c perRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{metadataKey: c.key}, nil
}

// RequireTransportSecurity is false because this demo runs over plaintext
// connections; a production system sending API keys would require TLS.
func (c perRPCCredentials) RequireTransportSecurity() bool {
	return false
}
