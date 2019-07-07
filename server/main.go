package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/ocagent"
	"contrib.go.opencensus.io/exporter/stackdriver"

	pb "github.com/DazWilkin/basic-personality/protos"
	"github.com/google/trillian"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"
	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz/service"
)

const (
	serviceName = "basic-personality-server"
)

var (
	projectId    = flag.String("project_id", "", "GCP Project ID for Stackdriver Trace.")
	grpcEndpoint = flag.String("grpc_endpoint", "", "The gRPC Endpoint to list to.")
	httpEndpoint = flag.String("http_endpoint", "", "The HTTP endpoint to listen to.")
	jeagEndpoint = flag.String("jeag_endpoint", "", "The Jaeger Agent Endpoint.")
	jeocEndpoint = flag.String("jeoc_endpoint", "", "The Jaeger Collector Endpoint.")
	ocagEndpoint = flag.String("ocag_endpoint", "", "The gRPC endpoint of the OpenCensus Agent.")
	zpgzEndpoint = flag.String("zpgz_endpoint", "", "The port to export zPages.")
	tLogEndpoint = flag.String("tlog_endpoint", "", "The gRPC endpoint of the Trillian Log Server.")
	tLogID       = flag.Int64("tlog_id", 0, "Trillian Log ID.")
)

func main() {
	log.Println("[main] Entered")
	flag.Parse()

	log.Printf("[main] Starting Stackdriver exporter [%s]\n", *ocagEndpoint)
	sd, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:    *projectId,
		MetricPrefix: fmt.Sprintf("test-%s", serviceName),
	})
	if err != nil {
		log.Fatalf("Failed to create the Stackdriver exporter: %v", err)
	}
	defer sd.Flush()

	view.RegisterExporter(sd)
	view.SetReportingPeriod(60 * time.Second)

	log.Printf("[main] Starting Jaeger exporter [agent:%s; collector:%s]\n", *jeagEndpoint, *jeocEndpoint)
	je, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint:     *jeagEndpoint,
		CollectorEndpoint: *jeocEndpoint,
		ServiceName:       fmt.Sprintf("test-%s", serviceName),
	})
	if err != nil {
		log.Fatalf("Failed to create the Jaeger exporter: %v", err)
	}

	// And now finally register it as a Trace Exporter
	trace.RegisterExporter(je)

	log.Printf("[main] Starting OpenCensus Agent exporter [%s]\n", *ocagEndpoint)
	oc, err := ocagent.NewExporter(
		ocagent.WithAddress(*ocagEndpoint),
		ocagent.WithInsecure(),
		ocagent.WithReconnectionPeriod(10*time.Second),
		ocagent.WithServiceName(serviceName),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer oc.Stop()

	view.RegisterExporter(oc)
	trace.RegisterExporter(oc)

	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Fatal(err)
	}

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

	// gRPC server utilizes default stats|trace handling w/ OpenCensus
	grpcServer := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))

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

	// zPages
	zPagesMux := http.NewServeMux()
	zpages.Handle(zPagesMux, "/debug")

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
	}()

	// zPages Server
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
		log.Printf("Starting zPages Listener [%s]\n", *zpgzEndpoint)
		log.Fatal(zpgzServer.Serve(listen))
	}()

	wg.Wait()
	log.Println("[main] Done")
}
