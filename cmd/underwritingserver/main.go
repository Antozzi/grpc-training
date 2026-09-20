package main

import (
	"io"
	"log"
	"net"

	"github.com/Antozzi/grpc-training/internal/authkey"
	"github.com/Antozzi/grpc-training/internal/interceptor"

	commonpb "github.com/Antozzi/grpc-training/gen/pb/common"
	underwritingpb "github.com/Antozzi/grpc-training/gen/pb/underwriting"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const port = ":50052"

type underwritingServer struct {
	underwritingpb.UnimplementedUnderwritingServiceServer
}

func (s *underwritingServer) SubmitMedicalHistory(stream underwritingpb.UnderwritingService_SubmitMedicalHistoryServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	appCtx := first.GetContext()
	if appCtx == nil {
		return status.Error(codes.InvalidArgument, "first message must carry an ApplicationContext")
	}

	var totalSeverity, recordCount int
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		record := req.GetRecord()
		if record == nil {
			return status.Error(codes.InvalidArgument, "messages after the first must carry a MedicalRecord")
		}
		totalSeverity += int(record.GetSeverity())
		recordCount++
	}

	avgSeverity := 0.0
	if recordCount > 0 {
		avgSeverity = float64(totalSeverity) / float64(recordCount)
	}

	// Simplified, illustrative scoring: not actuarial.
	approved := true
	finalTier := appCtx.GetBaseRiskTier()
	notes := "no material findings; base pricing stands"
	riskAdjustment := 1.0

	switch {
	case avgSeverity >= 8:
		approved = false
		finalTier = commonpb.RiskTier_RISK_TIER_DECLINED
		notes = "average reported severity too high to underwrite"
	case avgSeverity >= 5:
		finalTier = commonpb.RiskTier_RISK_TIER_ELEVATED
		riskAdjustment = 1.0 + avgSeverity*0.08
		notes = "elevated risk factors found; premium adjusted"
	case avgSeverity > 0:
		riskAdjustment = 1.0 + avgSeverity*0.08
		notes = "minor risk factors found; premium adjusted"
	}

	adjustedPremium := &commonpb.Money{
		CurrencyCode: appCtx.GetBaseMonthlyPremium().GetCurrencyCode(),
		MinorUnits:   int64(float64(appCtx.GetBaseMonthlyPremium().GetMinorUnits()) * riskAdjustment),
	}
	if !approved {
		adjustedPremium = appCtx.GetBaseMonthlyPremium()
	}

	return stream.SendAndClose(&underwritingpb.UnderwritingDecision{
		QuoteId:                appCtx.GetQuoteId(),
		Approved:               approved,
		FinalRiskTier:          finalTier,
		AdjustedMonthlyPremium: adjustedPremium,
		Notes:                  notes,
	})
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
	underwritingpb.RegisterUnderwritingServiceServer(s, &underwritingServer{})
	reflection.Register(s)

	log.Printf("UnderwritingService listening on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
