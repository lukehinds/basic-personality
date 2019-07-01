package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/trillian"
	"github.com/google/trillian/merkle/rfc6962"
	"github.com/google/trillian/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
func (s *server) put(r *PutRequest) (*PutResponse, error) {
	log.Println("[server:put] Entered")

	// Marshal a Thing (actually just 'name' which is a string) into []byte
	// Eventually we'll marshal a more interesting data structure
	leafValue, err := r.thing.Marshal()
	if err != nil {
		log.Fatal(err)
	}
	// Marshal an Extra (again)
	extraData, err := r.extra.Marshal()
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
		return &PutResponse{}, fmt.Errorf("[server:put] Bad status: %v", resp.QueuedLeaf.GetStatus())
	}
	if c == codes.OK {
		log.Println("[server:put] ok")
	} else if c == codes.AlreadyExists {
		log.Printf("[server:put] %s already Exists", leafValue)
	}

	return &PutResponse{
		status: "ok",
	}, nil
}
func (s *server) get(r *GetRequest) (*GetResponse, error) {
	log.Println("[server:get] Entered")

	// Marshal a Thing (actually just 'name' which is a string) into []byte
	// Eventually we'll marshal a more interesting data structure
	leafValue, err := r.thing.Marshal()
	if err != nil {
		log.Fatal(err)
	}

	// Trillian uses its own (rfc6962) hasher
	hasher := rfc6962.DefaultHasher
	leafHash := hasher.HashLeaf(leafValue)
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
		return &GetResponse{}, err
	}

	// Iterate over the responses; there should be 0 or 1
	result := &GetResponse{}
	for i, logLeaf := range resp.GetLeaves() {
		result.value = logLeaf.GetLeafValue()
		result.index = logLeaf.GetLeafIndex()
		log.Printf("[server:get] %d: %d %s", i, result.index, result.value)
	}

	return result, nil
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
func (s *server) proof(r *ProofRequest) (*ProofResponse, error) {
	log.Println("[server:proof] Entered")

	root, err := s.root()
	if err != nil {
		return &ProofResponse{}, err
	}
	log.Printf("[server:proof] Root hash: %x", root.RootHash)

	rqst := &trillian.GetInclusionProofRequest{
		LogId:     s.logID,
		LeafIndex: r.index,
		TreeSize:  int64(root.TreeSize),
	}
	resp, err := s.client.GetInclusionProof(context.Background(), rqst)
	if st := status.Convert(err); st.Code() != codes.OK {
		return &ProofResponse{}, status.Errorf(st.Code(), "Cannot fetch log inclusion proof")
	}
	hashes := resp.GetProof().GetHashes()
	return &ProofResponse{
		hashes: hashes,
	}, nil
}
