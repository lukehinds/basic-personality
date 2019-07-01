package main

// PutRequest is a convenience type that we'll extend in the next iteration
type PutRequest struct {
	thing Thing
	extra Extra
}

// PutResponse is a convenience type that we'll extend in the next iteration
type PutResponse struct {
	status string
}

// GetRequest is a convenience type that we'll extend in the next iteration
type GetRequest struct {
	thing Thing
	extra Extra
}

// GetResponse is a convenience type that we'll extend in the next iteration
type GetResponse struct {
	status string
	value  []byte
	index  int64
}

// ProofRequest is a convenience type that we'll extend in the next iteration
type ProofRequest struct {
	index int64
}

// ProofResponse is a convenience type that we'll extend in the next iteration
type ProofResponse struct {
	hashes [][]byte
}
