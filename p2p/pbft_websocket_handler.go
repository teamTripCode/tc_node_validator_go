package p2p

import (
	"encoding/json" // Added for unmarshaling consensus messages
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"tripcodechain_go/consensus"      // Still needed for consensus.Consensus and consensus.PBFT
	"tripcodechain_go/pkg/consensus_types" // Added for consensus_types.Message
	"tripcodechain_go/utils"
)

// upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for now
	},
}

// pbftClients stores active WebSocket connections for PBFT.
var pbftClients = make(map[*websocket.Conn]bool)
var pbftClientsMutex = sync.RWMutex{}

// StartPBFTWebsocketServer starts the WebSocket server for PBFT communication.
// It now accepts a consensus.Consensus interface to pass messages to the PBFT engine.
func StartPBFTWebsocketServer(nodeID string, port int, pbftConsensus consensus.Consensus) {
	utils.LogInfo("[PBFT_WS] Attempting to start PBFT WebSocket server for node %s on port %d", nodeID, port)

	http.HandleFunc("/pbft", func(w http.ResponseWriter, r *http.Request) {
		// pbftConsensus is available here due to closure
		pbftWsHandler(w, r, nodeID, pbftConsensus)
	})

	addr := fmt.Sprintf(":%d", port)
	utils.LogInfo("[PBFT_WS] Listening on %s/pbft", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("[PBFT_WS] Failed to start PBFT WebSocket server: %v", err)
	}
}

// pbftWsHandler handles incoming WebSocket connections for PBFT.
// It now takes pbftConsensus to process messages.
func pbftWsHandler(w http.ResponseWriter, r *http.Request, serverNodeID string, pbftConsensus consensus.Consensus) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.LogError("[PBFT_WS] Failed to upgrade connection for %s: %v", r.RemoteAddr, err)
		return
	}
	defer conn.Close()

	pbftClientsMutex.Lock()
	pbftClients[conn] = true
	pbftClientsMutex.Unlock()

	utils.LogInfo("[PBFT_WS] Connection established with %s", r.RemoteAddr)
	LogPBFTConnectionEstablished(serverNodeID, r.RemoteAddr, time.Duration(0))

	defer func() {
		pbftClientsMutex.Lock()
		delete(pbftClients, conn)
		pbftClientsMutex.Unlock()
		utils.LogInfo("[PBFT_WS] Connection closed with %s", r.RemoteAddr)
	}()

	for {
		_, messageBytes, err := conn.ReadMessage() // messageType is _
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				utils.LogError("[PBFT_WS] Error reading message from %s: %v", r.RemoteAddr, err)
			} else {
				utils.LogInfo("[PBFT_WS] Client %s disconnected.", r.RemoteAddr)
			}
			break
		}

		var msg consensus_types.Message // Changed to consensus_types.Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			utils.LogError("[PBFT_WS] Failed to unmarshal PBFT message from %s: %v. Message: %s", r.RemoteAddr, err, string(messageBytes))
			continue
		}

		utils.LogDebug("[PBFT_WS] Received %s message from %s", msg.Type, conn.RemoteAddr().String())

		pbftInstance, ok := pbftConsensus.(*consensus.PBFT)
		if !ok {
			utils.LogError("[PBFT_WS] PBFT consensus instance is not of type *PBFT for node %s. Cannot process message.", serverNodeID)
			continue // Or handle more gracefully, maybe this server shouldn't run if consensus is wrong type
		}

		if err := pbftInstance.ProcessConsensusMessage(&msg); err != nil {
			utils.LogError("[PBFT_WS] Error processing PBFT message type %s from %s: %v", msg.Type, r.RemoteAddr, err)
			continue
		}
		// Successfully processed message
		utils.LogDebug("[PBFT_WS] Successfully processed %s message from %s", msg.Type, conn.RemoteAddr().String())
	}
}

// BroadcastPBFTMessage sends a message to all connected PBFT WebSocket clients.
func BroadcastPBFTMessage(message []byte) {
	pbftClientsMutex.RLock()
	defer pbftClientsMutex.RUnlock()

	if len(pbftClients) == 0 {
		// utils.LogInfo("[PBFT_WS] No PBFT clients connected, cannot broadcast message.") // Can be noisy
		return
	}

	// utils.LogDebug("[PBFT_WS] Broadcasting message to %d PBFT clients.", len(pbftClients)) // Can be noisy
	for client := range pbftClients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			utils.LogError("[PBFT_WS] Error writing message to client %s: %v", client.RemoteAddr().String(), err)
		}
	}
}

// LogPBFTConnectionEstablished logs when a PBFT WebSocket connection is established.
func LogPBFTConnectionEstablished(nodeID string, connectedNodeID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBFT_connection_established - Node:%s (connectedTo:%s) - Latency:%dms", timestamp, nodeID, connectedNodeID, latencyMs)
}

// LogPBFTLeaderElection logs when a new leader is elected in the PBFT process.
func LogPBFTLeaderElection(nodeID string, electedLeaderID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBFT_leader_election - Node:%s (electedLeader:%s) - Latency:%dms", timestamp, nodeID, electedLeaderID, latencyMs)
}

// LogPBFTViewChange logs when a view change occurs in the PBFT process.
// newViewID is assumed to be a string representation of the view identifier.
func LogPBFTViewChange(nodeID string, newViewID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBFT_view_change - Node:%s (newView:%s) - Latency:%dms", timestamp, nodeID, newViewID, latencyMs)
}

// LogPBFTConsensusReached logs when consensus is reached on a block in the PBFT process.
func LogPBFTConsensusReached(nodeID string, blockID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBFT_consensus_reached - Node:%s (blockID:%s) - Latency:%dms", timestamp, nodeID, blockID, latencyMs)
}
