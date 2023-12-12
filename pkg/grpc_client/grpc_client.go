package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	pb "github.com/wizhaoredhat/dpu-operator/pkg/plugin/generated/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

var (
	node_name string
)

func main() {
	var grpcPort int
	flag.IntVar(&grpcPort, "grpc_port", 50151, "The gRPC server port")

	flag.Parse()

	node_name = os.Getenv("NODE_NAME")

	klog.Info("Starting main loop gRPC client with port", grpcPort)
	runGrpcClient(grpcPort)
}

func periodicGetVersion(ctx context.Context, ch chan struct{}, c pb.DpuConfigSrvClient) {
	r, err := c.GetVersion(ctx, &pb.VersionRequest{ComponentName: "DPU on" + node_name})
	if err != nil {
		log.Fatalf("could not get version: %v", err)
	}
	log.Printf("(%s) Got version: %s", node_name, r.GetMessage())
	time.Sleep(3 * time.Second)
	ch <- struct{}{}
}

func runGrpcClient(grpcPort int) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(fmt.Sprint("localhost:", grpcPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewDpuConfigSrvClient(conn)

	// Contact the server and print out its response.
	ctx := context.Background()

	wait := make(chan struct{})
	for {
		go periodicGetVersion(ctx, wait, c)
		<-wait
	}
}
