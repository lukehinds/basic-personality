package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/trillian"

	"google.golang.org/grpc"
)

const (
	serviceName = "image-transparency-server"
)

var (
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

	// Eventually this personality will be a server
	log.Printf("[main] Creating Server using LogID [%d]", *tLogID)
	server := newServer(tLogClient, *tLogID)

	// Leaves comprise a primary LeafValue (thing) and may have associated ExtraData(extra)
	// The LeafValue will become the hashed value for a node in the Merkle Tree
	log.Println("[main] Creating a 'Thing' and something 'Extra'")
	thing := newThing(fmt.Sprintf("[%s] Thing", time.Now().Format(time.RFC3339)))
	extra := newExtra("Extra")

	var wg sync.WaitGroup

	// Try to put this Request (Thing+Extra) in the Log
	log.Println("[main] Submitting it for inclusion in the Trillian Log")
	wg.Add(1)
	go func() {
		defer func() {
			log.Println("[main:put] Done")
			wg.Done()
		}()
		log.Println("[main:put] Entered")
		resp, err := server.put(&Request{
			thing: *thing,
			extra: *extra,
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("[main:put] Status:%s", resp.status)
	}()

	// Await the Inclusion (Proof)
	log.Println("[main] Awaiting Inclusion (Proof) in the Trillian Log")
	wg.Add(1)
	go func() {
		defer func() {
			log.Println("[main:wait] Done")
			wg.Done()
		}()
		log.Println("[main:wait] Entered")
		for {
			resp, err := server.wait(&Request{
				thing: *thing,
			})
			if err != nil {
				log.Printf("[main:wait] %s", err)
			}
			log.Printf("[main:wait] Status:%s", resp.status)
			if resp.status == "ok" {
				break
			}
			log.Println("[main:wait] Sleeping")
			time.Sleep(1 * time.Second)
		}
	}()

	// Try to get this Request (Thing+Extra) from the Log
	log.Println("[main] Retrieving it from the Trillian Log")
	wg.Add(1)
	go func() {
		defer func() {
			log.Println("[main:get] Done")
			wg.Done()
		}()
		log.Println("[main:get] Entered")
		for {
			resp, err := server.get(&Request{
				thing: *thing,
				extra: *extra,
			})
			if err != nil {
				log.Printf("[main:get] %s", err)
			}
			log.Printf("[main:get] Status:%s", resp.status)
			if resp.status == "ok" {
				break
			}
			log.Println("[main:get] Sleeping")
			time.Sleep(1 * time.Second)
		}
	}()

	wg.Wait()
	log.Println("[main] Done")
}
