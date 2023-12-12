package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/wizhaoredhat/dpu-operator/pkg/plugin/generated/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"
)

var (
	node_name string
)

type server struct {
	pb.UnimplementedDpuConfigSrvServer
}

func (s *server) GetVersion(ctx context.Context, in *pb.VersionRequest) (*pb.VersionReply, error) {
	log.Printf("(%s) Received: %v", node_name, in.GetComponentName())
	return &pb.VersionReply{Message: "Version 0.1 for " + in.GetComponentName() + " " + node_name}, nil
}

func main() {
	var grpcPort int
	flag.IntVar(&grpcPort, "grpc_port", 50151, "The gRPC server port")

	flag.Parse()

	node_name = os.Getenv("NODE_NAME")

	klog.Info("Starting gRPC Server with port", grpcPort)
	runGrpcServer(grpcPort)
}

// This function will enable the use of gRPC curl
// https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md
func enable_grpc_curl(s *grpc.Server) {
	reflection.Register(s)
}

func runGrpcServer(grpcPort int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterDpuConfigSrvServer(s, &server{})

	enable_grpc_curl(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
