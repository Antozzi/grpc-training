package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/Antozzi/grpc-training/internal/authkey"
	"github.com/Antozzi/grpc-training/internal/idgen"
	"github.com/Antozzi/grpc-training/internal/interceptor"

	commonpb "github.com/Antozzi/grpc-training/gen/pb/common"
	quotepb "github.com/Antozzi/grpc-training/gen/pb/quote"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const port = ":50051"

var validTermYears = map[int32]bool{10: true, 15: true, 20: true, 30: true}

type quoteServer struct {
	quotepb.UnimplementedQuoteServiceServer
}

func (s *quoteServer) GetQuote(_ context.Context, req *quotepb.QuoteRequest) (*quotepb.QuoteResponse, error) {
	if req.GetAge() < 18 || req.GetAge() > 80 {
		return nil, status.Error(codes.InvalidArgument, "age must be between 18 and 80")
	}
	if req.GetCoverageAmount().GetMinorUnits() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "coverage_amount must be positive")
	}
	if !validTermYears[req.GetTermYears()] {
		return nil, status.Error(codes.InvalidArgument, "term_years must be one of 10, 15, 20, 30")
	}

	premium, tier := computePremium(req)

	return &quotepb.QuoteResponse{
		QuoteId: idgen.New("quo"),
		MonthlyPremium: &commonpb.Money{
			CurrencyCode: "USD",
			MinorUnits:   premium,
		},
		RiskTier:  tier,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}, nil
}

// computePremium is a simplified, illustrative pricing model, not an
// actuarial one: it scales a flat base rate per $1,000 of coverage by age,
// smoking status, and term length.
func computePremium(req *quotepb.QuoteRequest) (minorUnits int64, tier commonpb.RiskTier) {
	coverageThousands := float64(req.GetCoverageAmount().GetMinorUnits()) / 100.0 / 1000.0
	const baseRatePerThousand = 0.35 // USD per month, per $1,000 of coverage

	ageFactor := 1.0 + float64(req.GetAge()-30)*0.03
	if ageFactor < 0.5 {
		ageFactor = 0.5
	}
	smokerFactor := 1.0
	if req.GetSmoker() {
		smokerFactor = 1.75
	}
	termFactor := 1.0 + float64(req.GetTermYears()-10)*0.01

	monthlyUSD := coverageThousands * baseRatePerThousand * ageFactor * smokerFactor * termFactor
	minorUnits = int64(monthlyUSD * 100)

	tier = commonpb.RiskTier_RISK_TIER_STANDARD
	switch {
	case req.GetAge() >= 65 || (req.GetSmoker() && req.GetAge() >= 50):
		tier = commonpb.RiskTier_RISK_TIER_ELEVATED
	case req.GetAge() < 40 && !req.GetSmoker():
		tier = commonpb.RiskTier_RISK_TIER_LOW
	}
	return minorUnits, tier
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
	quotepb.RegisterQuoteServiceServer(s, &quoteServer{})
	reflection.Register(s)

	log.Printf("QuoteService listening on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
