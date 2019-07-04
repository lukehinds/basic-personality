package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/trillian"
	"github.com/google/trillian/merkle"
	"github.com/google/trillian/merkle/rfc6962"
	"github.com/google/trillian/types"
	"google.golang.org/grpc/codes"

	pb "github.com/DazWilkin/basic-personality/protos"
	"github.com/gogo/protobuf/proto"
)

type server struct {
	client trillian.TrillianLogClient
	logID  int64
}

func newServer(client trillian.TrillianLogClient, logID int64) *server {
	log.Println("[server] Creating")
	return &server{
		client: client,
		logID:  logID,
	}
}
func (s *server) PutThing(ctx context.Context, r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Printf("PutThing")
	resp, err := s.put(r)
	return resp, err
}
func (s *server) GetThing(ctx context.Context, r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Printf("GetThing")
	resp, err := s.get(r)
	return resp, err
}
func (s *server) WaitThing(ctx context.Context, r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Printf("Wait")
	resp, err := s.wait(r)
	return resp, err
}
func (s *server) put(r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Println("[server:put] Entered")

	// Marshal a Thing (actually just 'name' which is a string) into []byte
	// Eventually we'll marshal a more interesting data structure
	leafValue, err := proto.Marshal(r.GetThing())
	if err != nil {
		log.Fatal(err)
	}
	// Marshal an Extra (again)
	extraData, err := proto.Marshal(r.GetExtra())
	if err != nil {
		log.Fatal(err)
	}
	leaf := &trillian.LogLeaf{
		LeafValue: leafValue,
		ExtraData: extraData,
	}
	rqst := &trillian.QueueLeafRequest{
		LogId: s.logID,
		Leaf:  leaf,
	}
	resp, err := s.client.QueueLeaf(context.Background(), rqst)
	if err != nil {
		log.Fatal(err)
	}

	c := codes.Code(resp.QueuedLeaf.GetStatus().GetCode())
	if c != codes.OK && c != codes.AlreadyExists {
		return &pb.ThingResponse{}, fmt.Errorf("[server:put] Bad status: %v", resp.QueuedLeaf.GetStatus())
	}
	var status string
	if c == codes.OK {
		status = "ok"
	} else if c == codes.AlreadyExists {
		status = fmt.Sprintf("LeafValue: %s already exists", leafValue)
	}

	return &pb.ThingResponse{
		Status: status,
	}, nil
}
func (s *server) root() (types.LogRootV1, error) {
	rqst := &trillian.GetLatestSignedLogRootRequest{
		LogId: s.logID,
	}
	resp, err := s.client.GetLatestSignedLogRoot(context.Background(), rqst)
	if err != nil {
		return types.LogRootV1{}, err
	}
	var root types.LogRootV1
	if err := root.UnmarshalBinary(resp.SignedLogRoot.LogRoot); err != nil {
		return types.LogRootV1{}, err
	}
	return root, nil
}
func (s *server) wait(r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Println("[server:wait] Entered")

	root, err := s.root()
	if err != nil {
		return &pb.ThingResponse{}, err
	}
	log.Printf("[server:wait] Root hash: %x", root.RootHash)

	leafValue, err := proto.Marshal(r.GetThing())
	if err != nil {
		log.Fatal(err)
	}

	// Trillian uses its own (rfc6962) hasher
	hasher := rfc6962.DefaultHasher
	leafHash, err := hasher.HashLeaf(leafValue)
	if err != nil {
		return &pb.ThingResponse{}, err
	}

	rqst := &trillian.GetInclusionProofByHashRequest{
		LogId:    s.logID,
		LeafHash: leafHash,
		TreeSize: int64(root.TreeSize),
	}

	resp, err := s.client.GetInclusionProofByHash(context.Background(), rqst)
	if err != nil {
		return &pb.ThingResponse{}, err
	}
	if len(resp.Proof) < 1 {
		return &pb.ThingResponse{}, nil
	}

	v := merkle.NewLogVerifier(rfc6962.DefaultHasher)
	for i, proof := range resp.Proof {
		hashes := proof.GetHashes()
		for j, hash := range hashes {
			log.Printf("[main] proof[%d],hash[%d] == %x\n", i, j, hash)
		}
		if err := v.VerifyInclusionProof(proof.LeafIndex, int64(root.TreeSize), hashes, root.RootHash, leafHash); err != nil {
			return &pb.ThingResponse{}, err
		}
	}
	return &pb.ThingResponse{
		Status: "ok",
	}, nil
}
func (s *server) get(r *pb.ThingRequest) (*pb.ThingResponse, error) {
	log.Println("[server:get] Entered")

	// Marshal a Thing (actually just 'name' which is a string) into []byte
	// Eventually we'll marshal a more interesting data structure
	leafValue, err := proto.Marshal(r.GetThing())
	if err != nil {
		log.Fatal(err)
	}

	// Trillian uses its own (rfc6962) hasher
	hasher := rfc6962.DefaultHasher
	leafHash, err := hasher.HashLeaf(leafValue)
	if err != nil {
		return &pb.ThingResponse{}, err
	}
	// Output the hashed value (conventionally hex is used)
	log.Printf("[server:get] hash: %x\n", leafHash)

	// Create the request
	rqst := &trillian.GetLeavesByHashRequest{
		LogId:    s.logID,
		LeafHash: [][]byte{leafHash},
	}

	// Submit the request to the Trillian Log Server
	resp, err := s.client.GetLeavesByHash(context.Background(), rqst)
	if err != nil {
		return &pb.ThingResponse{}, err
	}

	// Iterate over the responses; there should be 0 or 1
	for i, logLeaf := range resp.GetLeaves() {
		log.Printf("[server:get] %d: %d %s", i, logLeaf.GetLeafIndex(), logLeaf.GetLeafValue())
	}

	return &pb.ThingResponse{
		Status: "ok",
	}, nil
}
