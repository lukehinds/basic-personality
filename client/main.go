package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	pb "github.com/DazWilkin/basic-personality/protos"
	"google.golang.org/grpc"
)

var (
	grpcEndpoint = flag.String("grpc_endpoint", "", "The gRPC endpoint to dial.")
)

func main() {
	flag.Parse()

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}
	conn, err := grpc.Dial(*grpcEndpoint, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewBasicPersonalityClient(conn)
	healthClient := pb.NewHealthClient(conn)

	ctx := context.Background()

	// Subscribe to healthchecks
	stream, err := healthClient.Watch(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			s, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("healthcheck: %s", s)
		}
	}()

	for {
		// PutThing
		go func() {
			startTime := time.Now()
			rqst := &pb.ThingRequest{}
			resp, err := client.PutThing(ctx, rqst)
			if err != nil {
				log.Fatal(err)
			}
			latencyMs := float64(time.Since(startTime)) / 1e6
			log.Printf("PutThing Latency: %f", latencyMs)
			log.Printf("%v", resp)

		}()
		// GetThing
		go func() {
			startTime := time.Now()
			rqst := &pb.ThingRequest{}
			resp, err := client.GetThing(ctx, rqst)
			if err != nil {
				log.Fatal(err)
			}
			latencyMs := float64(time.Since(startTime)) / 1e6
			log.Printf("GetThing Latency: %f", latencyMs)
			log.Printf("%v", resp)
		}()
		time.Sleep(5 * time.Second)
	}
}
