package main

import (
	"context"
	"flag"
	"fmt"
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
		log.Println("[main] Creating a 'Thing' and something 'Extra'")
		thing := &pb.ThingRequest{
			Thing: &pb.Thing{
				Name: fmt.Sprintf("[%s] Thing", time.Now().Format(time.RFC3339)),
			},
			Extra: &pb.Extra{
				Name: "Extra",
			},
		}
		// PutThing
		go func() {
			startTime := time.Now()
			resp, err := client.PutThing(ctx, thing)
			if err != nil {
				log.Fatal(err)
			}
			latencyMs := float64(time.Since(startTime)) / 1e6
			log.Printf("PutThing Latency: %f", latencyMs)
			log.Printf("%v", resp)

		}()
		// WaitThing
		go func() {
			for {
				startTime := time.Now()
				resp, err := client.WaitThing(ctx, thing)
				if err != nil {
					// Not fatal but anticipated
					log.Println(err)
				}
				latencyMs := float64(time.Since(startTime)) / 1e6
				log.Printf("GetThing Latency: %f", latencyMs)
				log.Printf("%v", resp)
				if resp.GetStatus() == "ok" {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}()

		// GetThing
		go func() {
			for {
				startTime := time.Now()
				resp, err := client.GetThing(ctx, thing)
				if err != nil {
					// Not fatal but anticipated
					log.Println(err)
				}
				latencyMs := float64(time.Since(startTime)) / 1e6
				log.Printf("GetThing Latency: %f", latencyMs)
				log.Printf("%v", resp)
				if resp.GetStatus() == "ok" {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}()
		time.Sleep(5 * time.Second)
	}
}
