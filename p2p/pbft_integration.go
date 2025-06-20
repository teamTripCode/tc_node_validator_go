package p2p

import (
	"fmt"
	"time"
	"tripcodechain_go/pkg/consensus_events"
	"tripcodechain_go/utils"
)

// min returns the smaller of x or y.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PBFTBroadcaster handles broadcasting PBFT messages.
type PBFTBroadcaster struct {
	NodeInstance *Node // Reference to the main p2p.Node
}

// NewPBFTBroadcaster creates a new PBFTBroadcaster.
func NewPBFTBroadcaster(node *Node) *PBFTBroadcaster {
	return &PBFTBroadcaster{NodeInstance: node}
}

// BroadcastPBFTMessage sends a PBFT message to all known peers.
// Note: This is a simplified broadcast. A real implementation might need
// to manage PBFT-specific peer lists or use a more robust broadcast mechanism.
func (b *PBFTBroadcaster) BroadcastPBFTMessage(data []byte) error {
	if b.NodeInstance == nil {
		return fmt.Errorf("PBFTBroadcaster: NodeInstance is nil")
	}

	b.NodeInstance.mutex.RLock()
	knownNodesCopy := make([]NodeStatus, 0, len(b.NodeInstance.knownNodes))
	for _, status := range b.NodeInstance.knownNodes {
		knownNodesCopy = append(knownNodesCopy, status)
	}
	b.NodeInstance.mutex.RUnlock()

	if len(knownNodesCopy) == 0 {
		utils.LogInfo("PBFTBroadcaster: No known nodes to broadcast PBFT message to.")
		return nil
	}

	utils.LogInfo("PBFTBroadcaster: Broadcasting PBFT message to %d known nodes.", len(knownNodesCopy))
	// TODO: Define a specific endpoint for generic PBFT messages, e.g., /pbft/message
	// For now, this is a placeholder and won't actually send.
	// A proper implementation would iterate and send HTTP POST requests.
	// Example:
	// for _, nodeStatus := range knownNodesCopy {
	//     if nodeStatus.Address == b.NodeInstance.ID {
	//         continue // Don't send to self
	//     }
	//     go func(addr string, msgData []byte) {
	//         url := fmt.Sprintf("http://%s/pbft/message", addr) // Replace with actual PBFT endpoint
	//         req, err := http.NewRequest("POST", url, bytes.NewBuffer(msgData))
	//         if err != nil {
	//             utils.LogError("PBFTBroadcaster: Error creating request to %s: %v", addr, err)
	//             return
	//         }
	//         req.Header.Set("Content-Type", "application/json")
	//         // Add any other necessary headers, like sender ID or signature
	//
	//         client := &http.Client{Timeout: 5 * time.Second}
	//         resp, err := client.Do(req)
	//         if err != nil {
	//             utils.LogError("PBFTBroadcaster: Error sending PBFT message to %s: %v", addr, err)
	//             return
	//         }
	//         defer resp.Body.Close()
	//         if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
	//             utils.LogError("PBFTBroadcaster: PBFT message rejected by %s, status: %s", addr, resp.Status)
	//         } else {
	//             utils.LogDebug("PBFTBroadcaster: PBFT message sent to %s, status: %s", addr, resp.Status)
	//         }
	//     }(nodeStatus.Address, data)
	// }
	utils.LogInfo("PBFTBroadcaster: Placeholder - actual HTTP broadcast not yet implemented. Message data (first 100 bytes): %s", string(data[:min(len(data), 100)]))

	return nil
}

// PBFTLogger handles logging PBFT-specific events.
type PBFTLogger struct{}

// NewPBFTLogger creates a new PBFTLogger.
func NewPBFTLogger() *PBFTLogger {
	return &PBFTLogger{}
}

// LogPBFTConsensusReached logs when PBFT consensus is reached for a block.
func (l *PBFTLogger) LogPBFTConsensusReached(nodeID, blockHash string, duration time.Duration) {
	utils.LogInfo("PBFTLogger: Consensus reached for block %s by node %s (duration: %v)", blockHash, nodeID, duration)
}

// LogPBFTViewChange logs when a PBFT view change occurs.
func (l *PBFTLogger) LogPBFTViewChange(nodeID, view string, duration time.Duration) {
	utils.LogInfo("PBFTLogger: View change to %s initiated/detected by node %s (duration: %v)", view, nodeID, duration)
}

// Ensure PBFTBroadcaster and PBFTLogger implement their respective interfaces
var _ consensus_events.ConsensusBroadcaster = (*PBFTBroadcaster)(nil)
var _ consensus_events.ConsensusLogger = (*PBFTLogger)(nil)
