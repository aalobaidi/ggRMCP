package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	proto "hello-service/proto"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

// helloServer implements the HelloService
type helloServer struct {
	proto.UnimplementedHelloServiceServer
}

// SayHello implements the SayHello RPC
func (s *helloServer) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	log.Printf("Received SayHello request: name=%s, email=%s", req.Name, req.Email)

	message := fmt.Sprintf("Hello %s! Your email is %s", req.Name, req.Email)

	return &proto.HelloReply{
		Message: message,
	}, nil
}

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	// Register the hello service
	proto.RegisterHelloServiceServer(s, &helloServer{})

	// Register reflection service on gRPC server
	reflection.Register(s)

	log.Printf("Hello service listening on port %d", *port)
	log.Printf("gRPC reflection enabled")
	log.Printf("HelloService registered")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
