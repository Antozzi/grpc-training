package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	pb "github.com/Antozzi/grpc-training/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "the address to connect to")
	name := flag.String("name", "world", "name to greet")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reply, err := client.SayHello(ctx, &pb.HelloRequest{Name: *name})
	if err != nil {
		log.Fatalf("SayHello failed: %v", err)
	}
	log.Printf("unary response: %s", reply.GetMessage())

	streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer streamCancel()

	stream, err := client.SayHelloStream(streamCtx, &pb.HelloRequest{Name: *name})
	if err != nil {
		log.Fatalf("SayHelloStream failed: %v", err)
	}
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("stream recv failed: %v", err)
		}
		log.Printf("stream response: %s", reply.GetMessage())
	}
}
