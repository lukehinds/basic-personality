package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/google/trillian"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"go.opencensus.io/zpages"

	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz/service"

	pb "github.com/DazWilkin/basic-personality/protos"
)

const (
	serviceName = "basic-personality-server"
)

var (
	grpcEndpoint = flag.String("grpc_endpoint", "", "The gRPC Endpoint to list to")
	httpEndpoint = flag.String("http_endpoint", "", "The HTTP endpoint to listen to.")
	zpgzEndpoint = flag.String("zpgz_endpoint", "", "The port to export zPages.")
	tLogEndpoint = flag.String("tlog_endpoint", "", "The gRPC endpoint of the Trillian Log Server.")
	tLogID       = flag.Int64("tlog_id", 0, "Trillian Log ID")
)

func main() {
	log.Println("[main] Entered")
	flag.Parse()

	// Establish gRPC connection w/ Trillian Log Server
	log.Printf("[main] Establishing connection w/ Trillian Log Server [%s]", *tLogEndpoint)
	conn, err := grpc.Dial(*tLogEndpoint, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a Trillian Log Server client
	log.Println("[main] Creating new Trillian Log Client")
	tLogClient := trillian.NewTrillianLogClient(conn)

	grpcServer := grpc.NewServer()

	// Channelz
	// See: https://grpc.io/blog/a_short_introduction_to_channelz/
	service.RegisterChannelzServiceToServer(grpcServer)

	pb.RegisterBasicPersonalityServer(grpcServer, newServer(tLogClient, *tLogID))
	pb.RegisterHealthServer(grpcServer, newHealthcheckServer())

	httpServeMux := http.NewServeMux()

	transcoder := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{
		grpc.WithInsecure(),
	}
	err = pb.RegisterBasicPersonalityHandlerFromEndpoint(context.Background(), transcoder, *httpEndpoint, dialOpts)
	if err != nil {
		log.Fatal(err)
	}
	httpServeMux.Handle("/api", transcoder)
	// httpServeMux.HandleFunc("/healthz", healthz)

	zPagesMux := http.NewServeMux()
	zpages.Handle(zPagesMux, "/")

	var wg sync.WaitGroup
	// gRPC Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		listen, err := net.Listen("tcp", *grpcEndpoint)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Starting gRPC Listener [%s]\n", *grpcEndpoint)
		log.Fatal(grpcServer.Serve(listen))
		// log.Fatal(http.ListenAndServeTLS(*grpcEndpoint, *tlsCrt, *tlsKey, grpcServer))
	}()

	wg.Wait()
	log.Println("[main] Done")
}
