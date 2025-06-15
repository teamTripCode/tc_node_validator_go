package p2p

// MCPMessageSignature defines the structure for a message signature.
type MCPMessageSignature struct {
	NodeID    string `json:"nodeId"`
	Signature string `json:"signature"`
}

// MCPQuery defines the structure for an MCP query message.
type MCPQuery struct {
	QueryID      string              `json:"queryId"`
	Timestamp    int64               `json:"timestamp"`
	OriginNodeID string              `json:"originNodeId"`
	Payload      []byte              `json:"payload"`
	Signature    MCPMessageSignature `json:"signature"`
}

// MCPResponse defines the structure for an MCP response message.
type MCPResponse struct {
	QueryID         string              `json:"queryId"`
	Timestamp       int64               `json:"timestamp"`
	OriginNodeID    string              `json:"originNodeId"` // Original querier
	ResponderNodeID string              `json:"responderNodeId"`
	Payload         []byte              `json:"payload"`
	Signature       MCPMessageSignature `json:"signature"`
}
