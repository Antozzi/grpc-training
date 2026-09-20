// Package interceptor holds server interceptors shared across every
// LifeSure service.
package interceptor

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLogging logs the method, duration, and resulting status code of
// every unary call.
func UnaryLogging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Printf("%s took=%s code=%s", info.FullMethod, time.Since(start), status.Code(err))
		return resp, err
	}
}

// StreamLogging logs the method, duration, and resulting status code of
// every streaming call, once the stream completes.
func StreamLogging() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		log.Printf("%s took=%s code=%s", info.FullMethod, time.Since(start), status.Code(err))
		return err
	}
}
