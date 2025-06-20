package llm

import "tripcodechain_go/p2p" // For p2p.MCPQuery and p2p.NodeManager types

// P2PBroadcaster defines the interface for broadcasting MCP queries
// and accessing necessary P2P node information.
// This interface is typically implemented by the P2P server
// and used by the DistributedLLMService to decouple them.
type P2PBroadcaster interface {
	BroadcastMCPQuery(query *p2p.MCPQuery)
	GetNodeID() string            // Used by LLMService to set OriginNodeID
	GetNodeMgr() *p2p.NodeManager // Used by LLMService for node management/awareness if needed beyond broadcasting
}
