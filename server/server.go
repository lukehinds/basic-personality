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
func (s *server) put(r *Request) (*Response, error) {
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
		return &Response{}, fmt.Errorf("[server:put] Bad status: %v", resp.QueuedLeaf.GetStatus())
	}
	var status string
	if c == codes.OK {
		status = "ok"
	} else if c == codes.AlreadyExists {
		status = fmt.Sprintf("LeafValue: %s already exists", leafValue)
	}

	return &Response{
		status: status,
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
func (s *server) wait(r *Request) (*Response, error) {
	log.Println("[server:wait] Entered")

	root, err := s.root()
	if err != nil {
		return &Response{}, err
	}
	log.Printf("[server:wait] Root hash: %x", root.RootHash)

	leafValue, err := r.thing.Marshal()
	if err != nil {
		log.Fatal(err)
	}

	// Trillian uses its own (rfc6962) hasher
	hasher := rfc6962.DefaultHasher
	leafHash, err := hasher.HashLeaf(leafValue)
	if err != nil {
		return &Response{}, err
	}

	rqst := &trillian.GetInclusionProofByHashRequest{
		LogId:    s.logID,
		LeafHash: leafHash,
		TreeSize: int64(root.TreeSize),
	}

	resp, err := s.client.GetInclusionProofByHash(context.Background(), rqst)
	if err != nil {
		return &Response{}, err
	}
	if len(resp.Proof) < 1 {
		return &Response{}, nil
	}

	v := merkle.NewLogVerifier(rfc6962.DefaultHasher)
	for i, proof := range resp.Proof {
		hashes := proof.GetHashes()
		for j, hash := range hashes {
			log.Printf("[main] proof[%d],hash[%d] == %x\n", i, j, hash)
		}
		if err := v.VerifyInclusionProof(proof.LeafIndex, int64(root.TreeSize), hashes, root.RootHash, leafHash); err != nil {
			return &Response{}, err
		}
	}
	return &Response{
		status: "ok",
	}, nil
}
func (s *server) get(r *Request) (*Response, error) {
	log.Println("[server:get] Entered")

	// Marshal a Thing (actually just 'name' which is a string) into []byte
	// Eventually we'll marshal a more interesting data structure
	leafValue, err := r.thing.Marshal()
	if err != nil {
		log.Fatal(err)
	}

	// Trillian uses its own (rfc6962) hasher
	hasher := rfc6962.DefaultHasher
	leafHash, err := hasher.HashLeaf(leafValue)
	if err != nil {
		return &Response{}, err
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
		return &Response{}, err
	}

	// Iterate over the responses; there should be 0 or 1
	for i, logLeaf := range resp.GetLeaves() {
		log.Printf("[server:get] %d: %d %s", i, logLeaf.GetLeafIndex(), logLeaf.GetLeafValue())
	}

	return &Response{
		status: "ok",
	}, nil
}
