package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/Antozzi/grpc-training/gen/pb"
	"google.golang.org/grpc"
)

const port = ":50051"

type greeterServer struct {
	pb.UnimplementedGreeterServer
}

func (s *greeterServer) SayHello(_ context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: fmt.Sprintf("Hello, %s!", req.GetName())}, nil
}

func (s *greeterServer) SayHelloStream(req *pb.HelloRequest, stream pb.Greeter_SayHelloStreamServer) error {
	for i := 1; i <= 5; i++ {
		reply := &pb.HelloReply{Message: fmt.Sprintf("Hello, %s! (%d/5)", req.GetName(), i)}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &greeterServer{})

	log.Printf("gRPC server listening on %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
