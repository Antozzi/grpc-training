package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/Antozzi/grpc-training/internal/authkey"
	"github.com/Antozzi/grpc-training/internal/idgen"
	"github.com/Antozzi/grpc-training/internal/interceptor"

	claimspb "github.com/Antozzi/grpc-training/gen/pb/claims"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const port = ":50054"

// requiredDocuments is how many documents this demo pipeline requires
// before a claim can be approved.
const requiredDocuments = 2

type claimsServer struct {
	claimspb.UnimplementedClaimsServiceServer
}

func (s *claimsServer) ProcessClaim(stream claimspb.ClaimsService_ProcessClaimServer) error {
	var claimID string
	var docCount int

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if claimID == "" && event.GetType() != claimspb.ClaimEventType_CLAIM_EVENT_TYPE_OPENED {
			return status.Error(codes.InvalidArgument, "first event on the stream must be CLAIM_EVENT_TYPE_OPENED")
		}

		var update *claimspb.ClaimStatusUpdate
		switch event.GetType() {
		case claimspb.ClaimEventType_CLAIM_EVENT_TYPE_OPENED:
			claimID = idgen.New("clm")
			update = &claimspb.ClaimStatusUpdate{
				ClaimId: claimID,
				Stage:   claimspb.ClaimStage_CLAIM_STAGE_RECEIVED,
				Message: "claim received, awaiting supporting documents",
			}

		case claimspb.ClaimEventType_CLAIM_EVENT_TYPE_DOCUMENT_SUBMITTED:
			docCount++
			update = &claimspb.ClaimStatusUpdate{
				ClaimId: claimID,
				Stage:   claimspb.ClaimStage_CLAIM_STAGE_UNDER_REVIEW,
				Message: fmt.Sprintf("document received (%d/%d), under review", docCount, requiredDocuments),
			}

		case claimspb.ClaimEventType_CLAIM_EVENT_TYPE_INFO_UPDATED:
			if docCount < requiredDocuments {
				update = &claimspb.ClaimStatusUpdate{
					ClaimId: claimID,
					Stage:   claimspb.ClaimStage_CLAIM_STAGE_ADDITIONAL_INFO_REQUIRED,
					Message: fmt.Sprintf("%d more document(s) needed", requiredDocuments-docCount),
				}
			} else {
				update = &claimspb.ClaimStatusUpdate{
					ClaimId: claimID,
					Stage:   claimspb.ClaimStage_CLAIM_STAGE_UNDER_REVIEW,
					Message: "information updated, review continuing",
				}
			}

		case claimspb.ClaimEventType_CLAIM_EVENT_TYPE_CLOSED:
			if docCount >= requiredDocuments {
				update = &claimspb.ClaimStatusUpdate{
					ClaimId: claimID,
					Stage:   claimspb.ClaimStage_CLAIM_STAGE_APPROVED,
					Message: "claim approved for payout",
				}
			} else {
				update = &claimspb.ClaimStatusUpdate{
					ClaimId: claimID,
					Stage:   claimspb.ClaimStage_CLAIM_STAGE_DENIED,
					Message: "insufficient documentation, claim denied",
				}
			}

		default:
			return status.Errorf(codes.InvalidArgument, "unknown claim event type: %v", event.GetType())
		}

		if err := stream.Send(update); err != nil {
			return err
		}
	}
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
	claimspb.RegisterClaimsServiceServer(s, &claimsServer{})
	reflection.Register(s)

	log.Printf("ClaimsService listening on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
