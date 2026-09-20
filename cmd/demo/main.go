// Command demo walks a single applicant through the full LifeSure flow:
// get a quote, submit medical history for underwriting, issue the policy if
// approved, list the holder's policies, then file and track a claim against
// it. It exercises all four gRPC service, all four RPC shapes (unary,
// client-streaming, server-streaming, bidirectional-streaming), and the
// shared auth/logging interceptors.
package main

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/Antozzi/grpc-training/internal/authkey"

	claimspb "github.com/Antozzi/grpc-training/gen/pb/claims"
	commonpb "github.com/Antozzi/grpc-training/gen/pb/common"
	policypb "github.com/Antozzi/grpc-training/gen/pb/policy"
	quotepb "github.com/Antozzi/grpc-training/gen/pb/quote"
	underwritingpb "github.com/Antozzi/grpc-training/gen/pb/underwriting"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	quoteAddr        = "localhost:50051"
	underwritingAddr = "localhost:50052"
	policyAddr       = "localhost:50053"
	claimsAddr       = "localhost:50054"

	holderID   = "holder-anton"
	holderName = "Anton Dyrdin"
)

func dial(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(authkey.NewCredentials(authkey.DemoKey)),
	)
	if err != nil {
		log.Fatalf("dial %s: %v", addr, err)
	}
	return conn
}

func main() {
	ctx := context.Background()

	// 1. QuoteService: unary.
	quoteConn := dial(quoteAddr)
	defer quoteConn.Close()
	quoteClient := quotepb.NewQuoteServiceClient(quoteConn)

	quoteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	quote, err := quoteClient.GetQuote(quoteCtx, &quotepb.QuoteRequest{
		Age:            45,
		Smoker:         false,
		CoverageAmount: &commonpb.Money{CurrencyCode: "USD", MinorUnits: 250_000_00},
		TermYears:      20,
	})
	cancel()
	if err != nil {
		log.Fatalf("GetQuote failed: %v", err)
	}
	log.Printf("quote %s: $%.2f/mo, risk tier %s", quote.GetQuoteId(),
		dollars(quote.GetMonthlyPremium()), quote.GetRiskTier())

	// 2. UnderwritingService: client-streaming.
	uwConn := dial(underwritingAddr)
	defer uwConn.Close()
	uwClient := underwritingpb.NewUnderwritingServiceClient(uwConn)

	uwCtx, uwCancel := context.WithTimeout(ctx, 5*time.Second)
	defer uwCancel()
	stream, err := uwClient.SubmitMedicalHistory(uwCtx)
	if err != nil {
		log.Fatalf("SubmitMedicalHistory failed: %v", err)
	}

	send(stream, &underwritingpb.SubmitMedicalHistoryRequest{
		Payload: &underwritingpb.SubmitMedicalHistoryRequest_Context{
			Context: &underwritingpb.ApplicationContext{
				QuoteId:            quote.GetQuoteId(),
				Age:                45,
				Smoker:             false,
				BaseMonthlyPremium: quote.GetMonthlyPremium(),
				BaseRiskTier:       quote.GetRiskTier(),
			},
		},
	})
	records := []*underwritingpb.MedicalRecord{
		{Type: underwritingpb.RecordType_RECORD_TYPE_CONDITION, Description: "controlled hypertension", Severity: 3},
		{Type: underwritingpb.RecordType_RECORD_TYPE_MEDICATION, Description: "lisinopril, daily", Severity: 2},
		{Type: underwritingpb.RecordType_RECORD_TYPE_LIFESTYLE, Description: "non-smoker, occasional alcohol", Severity: 1},
	}
	for _, record := range records {
		send(stream, &underwritingpb.SubmitMedicalHistoryRequest{
			Payload: &underwritingpb.SubmitMedicalHistoryRequest_Record{Record: record},
		})
	}

	decision, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("underwriting decision failed: %v", err)
	}
	log.Printf("underwriting decision: approved=%v tier=%s premium=$%.2f/mo (%s)",
		decision.GetApproved(), decision.GetFinalRiskTier(), dollars(decision.GetAdjustedMonthlyPremium()), decision.GetNotes())

	if !decision.GetApproved() {
		log.Fatal("application declined, stopping demo")
	}

	// 3. PolicyService: unary CreatePolicy, then server-streaming ListPolicies.
	policyConn := dial(policyAddr)
	defer policyConn.Close()
	policyClient := policypb.NewPolicyServiceClient(policyConn)

	policyCtx, policyCancel := context.WithTimeout(ctx, 5*time.Second)
	policy, err := policyClient.CreatePolicy(policyCtx, &policypb.CreatePolicyRequest{
		QuoteId:        quote.GetQuoteId(),
		HolderId:       holderID,
		HolderName:     holderName,
		MonthlyPremium: decision.GetAdjustedMonthlyPremium(),
		CoverageAmount: &commonpb.Money{CurrencyCode: "USD", MinorUnits: 250_000_00},
		TermYears:      20,
	})
	policyCancel()
	if err != nil {
		log.Fatalf("CreatePolicy failed: %v", err)
	}
	log.Printf("policy issued: %s status=%s $%.2f/mo", policy.GetPolicyId(), policy.GetStatus(), dollars(policy.GetMonthlyPremium()))

	listCtx, listCancel := context.WithTimeout(ctx, 5*time.Second)
	defer listCancel()
	listStream, err := policyClient.ListPolicies(listCtx, &policypb.ListPoliciesRequest{HolderId: holderID})
	if err != nil {
		log.Fatalf("ListPolicies failed: %v", err)
	}
	for {
		p, err := listStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("ListPolicies stream failed: %v", err)
		}
		log.Printf("holder %s owns policy %s (issued %s)", holderID, p.GetPolicyId(), p.GetIssuedAt())
	}

	// 4. ClaimsService: bidirectional streaming.
	claimsConn := dial(claimsAddr)
	defer claimsConn.Close()
	claimsClient := claimspb.NewClaimsServiceClient(claimsConn)

	claimCtx, claimCancel := context.WithTimeout(ctx, 5*time.Second)
	defer claimCancel()
	claimStream, err := claimsClient.ProcessClaim(claimCtx)
	if err != nil {
		log.Fatalf("ProcessClaim failed: %v", err)
	}

	events := []*claimspb.ClaimEvent{
		{PolicyId: policy.GetPolicyId(), Type: claimspb.ClaimEventType_CLAIM_EVENT_TYPE_OPENED, Detail: "hospitalization claim"},
		{PolicyId: policy.GetPolicyId(), Type: claimspb.ClaimEventType_CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED, Detail: "discharge summary"},
		{PolicyId: policy.GetPolicyId(), Type: claimspb.ClaimEventType_CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED, Detail: "itemized invoice"},
		{PolicyId: policy.GetPolicyId(), Type: claimspb.ClaimEventType_CLAIM_EVENT_TYPE_CLOSED, Detail: "all documents submitted"},
	}
	for _, event := range events {
		if err := claimStream.Send(event); err != nil {
			log.Fatalf("claim event send failed: %v", err)
		}
		update, err := claimStream.Recv()
		if err != nil {
			log.Fatalf("claim status recv failed: %v", err)
		}
		log.Printf("claim %s -> %s: %s", update.GetClaimId(), update.GetStage(), update.GetMessage())
	}
	_ = claimStream.CloseSend()

	log.Println("demo complete")
}

func send(stream underwritingpb.UnderwritingService_SubmitMedicalHistoryClient, req *underwritingpb.SubmitMedicalHistoryRequest) {
	if err := stream.Send(req); err != nil {
		log.Fatalf("stream send failed: %v", err)
	}
}

func dollars(m *commonpb.Money) float64 {
	return float64(m.GetMinorUnits()) / 100.0
}
