package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	pb "github.com/DazWilkin/basic-personality/protos"
	"go.opencensus.io/zpages"
	"google.golang.org/grpc"
)

var (
	grpcEndpoint = flag.String("grpc_endpoint", "", "The gRPC endpoint to dial.")
	zpgzEndpoint = flag.String("zpgz_endpoint", "", "The port to export zPages.")
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

	log.Println("[main] Creating a Basic-Personality Client")
	client := pb.NewBasicPersonalityClient(conn)
	log.Println("[main] Creating a Healthcheck Client")
	healthClient := pb.NewHealthClient(conn)

	// zPages
	zPagesMux := http.NewServeMux()
	zpages.Handle(zPagesMux, "/")

	// zPages Server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		zpgzServer := &http.Server{
			Addr:    *zpgzEndpoint,
			Handler: zPagesMux,
		}
		listen, err := net.Listen("tcp", *zpgzEndpoint)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("[main] Starting zPages Listener [%s]\n", *zpgzEndpoint)
		log.Fatal(zpgzServer.Serve(listen))
	}()
	defer wg.Wait()

	ctx := context.Background()

	// Subscribe to healthchecks
	log.Println("[main] Subscribing to Healthcheck stream")
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
			log.Printf("[main] Healthcheck: %s", s)
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
			log.Println("[main] Put the 'Thing' and 'Extra'")
			startTime := time.Now()
			resp, err := client.PutThing(ctx, thing)
			if err != nil {
				log.Fatal(err)
			}
			latencyMs := float64(time.Since(startTime)) / 1e6
			log.Printf("[main] Put Latency: %f", latencyMs)
			log.Printf("%v", resp)

		}()
		// WaitThing
		go func() {
			for {
				log.Println("[main] Wait the Inclusion Proof of the 'Thing'")
				startTime := time.Now()
				resp, err := client.WaitThing(ctx, thing)
				if err != nil {
					// Not fatal but anticipated
					log.Println(err)
				}
				latencyMs := float64(time.Since(startTime)) / 1e6
				log.Printf("[main] Wait Latency: %f", latencyMs)
				log.Printf("%v", resp)
				if resp.GetStatus() == "ok" {
					log.Println("[main] Wait Inclusion Proof done")
					break
				}
				log.Println("[main] Wait sleeping")
				time.Sleep(1 * time.Second)
			}
		}()
		// GetThing
		go func() {
			for {
				log.Println("[main] Get the 'Thing'")
				startTime := time.Now()
				resp, err := client.GetThing(ctx, thing)
				if err != nil {
					// Not fatal but anticipated
					log.Println(err)
				}
				latencyMs := float64(time.Since(startTime)) / 1e6
				log.Printf("[main] Get Latency: %f", latencyMs)
				log.Printf("%v", resp)
				if resp.GetStatus() == "ok" {
					log.Println("[main] Get done")
					break
				}
				log.Println("[main] Get sleeping")
				time.Sleep(1 * time.Second)
			}
		}()
		time.Sleep(15 * time.Second)
	}
}
