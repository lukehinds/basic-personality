package main

import (
	"context"
	"log"
	"time"

	pb "github.com/DazWilkin/basic-personality/protos"
)

type healthcheck struct{}

func newHealthcheckServer() *healthcheck {
	log.Println("Creating: healthcheck")
	return &healthcheck{}
}
func (hc *healthcheck) Check(ctx context.Context, in *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status: pb.HealthCheckResponse_SERVING,
	}, nil
}
func (hc *healthcheck) Watch(in *pb.HealthCheckRequest, stream pb.Health_WatchServer) error {
	for {
		log.Println("Healthcheck: Watch()")
		stream.Send(&pb.HealthCheckResponse{
			Status: pb.HealthCheckResponse_SERVING,
		})
		time.Sleep(15 * time.Second)
	}
}
