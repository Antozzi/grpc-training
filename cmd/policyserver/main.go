package main

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Antozzi/grpc-training/internal/authkey"
	"github.com/Antozzi/grpc-training/internal/idgen"
	"github.com/Antozzi/grpc-training/internal/interceptor"

	policypb "github.com/Antozzi/grpc-training/gen/pb/policy"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const port = ":50053"

// streamSendDelay simulates policies arriving incrementally, so ListPolicies
// visibly streams rather than delivering everything in one burst.
const streamSendDelay = 150 * time.Millisecond

type policyServer struct {
	policypb.UnimplementedPolicyServiceServer

	mu       sync.Mutex
	policies map[string]*policypb.Policy
}

func newPolicyServer() *policyServer {
	return &policyServer{policies: make(map[string]*policypb.Policy)}
}

func (s *policyServer) CreatePolicy(_ context.Context, req *policypb.CreatePolicyRequest) (*policypb.Policy, error) {
	if req.GetHolderId() == "" || req.GetHolderName() == "" {
		return nil, status.Error(codes.InvalidArgument, "holder_id and holder_name are required")
	}

	policy := &policypb.Policy{
		PolicyId:       idgen.New("pol"),
		HolderId:       req.GetHolderId(),
		HolderName:     req.GetHolderName(),
		MonthlyPremium: req.GetMonthlyPremium(),
		CoverageAmount: req.GetCoverageAmount(),
		TermYears:      req.GetTermYears(),
		Status:         "ACTIVE",
		IssuedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	s.policies[policy.GetPolicyId()] = policy
	s.mu.Unlock()

	return policy, nil
}

func (s *policyServer) GetPolicy(_ context.Context, req *policypb.GetPolicyRequest) (*policypb.Policy, error) {
	s.mu.Lock()
	policy, ok := s.policies[req.GetPolicyId()]
	s.mu.Unlock()
	if !ok {
		return nil, status.Errorf(codes.NotFound, "policy %q not found", req.GetPolicyId())
	}
	return policy, nil
}

func (s *policyServer) ListPolicies(req *policypb.ListPoliciesRequest, stream policypb.PolicyService_ListPoliciesServer) error {
	s.mu.Lock()
	var matches []*policypb.Policy
	for _, policy := range s.policies {
		if policy.GetHolderId() == req.GetHolderId() {
			matches = append(matches, policy)
		}
	}
	s.mu.Unlock()

	for _, policy := range matches {
		if err := stream.Send(policy); err != nil {
			return err
		}
		time.Sleep(streamSendDelay)
	}
	return nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authkey.UnaryServerInterceptor(authkey.DemoKey), interceptor.UnaryLogging()),
		grpc.ChainStreamInterceptor(authkey.StreamServerInterceptor(authkey.DemoKey), interceptor.StreamLogging()),
	)
	policypb.RegisterPolicyServiceServer(s, newPolicyServer())
	reflection.Register(s)

	log.Printf("PolicyService listening on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
