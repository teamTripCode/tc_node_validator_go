package p2p

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"tripcodechain_go/utils"
)

// pbosUpgrader is used to upgrade HTTP connections to WebSocket connections for pBOS.
var pbosUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for now
	},
}

// pbosClients stores active WebSocket connections for pBOS.
var pbosClients = make(map[*websocket.Conn]bool)
var pbosClientsMutex = sync.RWMutex{}

// StartPBOSWebsocketServer starts the WebSocket server for pBOS communication.
func StartPBOSWebsocketServer(nodeID string, port int) {
	utils.LogInfo("[pBOS_WS] Attempting to start pBOS WebSocket server for node %s on port %d", nodeID, port)

	http.HandleFunc("/pbos", func(w http.ResponseWriter, r *http.Request) {
		pbosWsHandler(w, r, nodeID) // Pass nodeID to the handler
	})

	addr := fmt.Sprintf(":%d", port)
	utils.LogInfo("[pBOS_WS] Listening on %s/pbos", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("[pBOS_WS] Failed to start pBOS WebSocket server: %v", err)
	}
}

// pbosWsHandler handles incoming WebSocket connections for pBOS.
func pbosWsHandler(w http.ResponseWriter, r *http.Request, serverNodeID string) {
	conn, err := pbosUpgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.LogError("[pBOS_WS] Failed to upgrade connection for %s: %v", r.RemoteAddr, err)
		return
	}
	defer conn.Close()

	// Add client to the map
	pbosClientsMutex.Lock()
	pbosClients[conn] = true
	pbosClientsMutex.Unlock()

	utils.LogInfo("[pBOS_WS] Connection established with %s", r.RemoteAddr)
	LogPBOSPeerJoined(serverNodeID, r.RemoteAddr, time.Duration(0))

	// Ensure client is removed from the map when the handler exits
	defer func() {
		pbosClientsMutex.Lock()
		delete(pbosClients, conn)
		pbosClientsMutex.Unlock()
		utils.LogInfo("[pBOS_WS] Connection closed with %s", r.RemoteAddr)
	}()

	// Placeholder loop for reading messages
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				utils.LogError("[pBOS_WS] Error reading message from %s: %v", r.RemoteAddr, err)
			} else {
				utils.LogInfo("[pBOS_WS] Client %s disconnected.", r.RemoteAddr)
			}
			break
		}
		// For now, just log received messages.
		log.Printf("[pBOS_WS] Received from %s (type %d): %s", r.RemoteAddr, messageType, message)
	}
}

// BroadcastPBOSMessage sends a message to all connected pBOS WebSocket clients.
func BroadcastPBOSMessage(message []byte) {
	pbosClientsMutex.RLock()
	defer pbosClientsMutex.RUnlock()

	if len(pbosClients) == 0 {
		utils.LogInfo("[pBOS_WS] No pBOS clients connected, cannot broadcast message.")
		return
	}

	utils.LogInfo("[pBOS_WS] Broadcasting message to %d pBOS clients.", len(pbosClients))
	for client := range pbosClients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			utils.LogError("[pBOS_WS] Error writing message to client %s: %v", client.RemoteAddr().String(), err)
		}
	}
}

// LogPBOSPeerJoined logs when a peer joins the pBOS network.
func LogPBOSPeerJoined(nodeID string, joinedPeerID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBOS_peer_joined - Node:%s (joinedPeer:%s) - Latency:%dms", timestamp, nodeID, joinedPeerID, latencyMs)
}

// LogPBOSTransactionBroadcast logs when a transaction is broadcast in the pBOS network.
func LogPBOSTransactionBroadcast(nodeID string, transactionID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBOS_transaction_broadcast - Node:%s (transactionID:%s) - Latency:%dms", timestamp, nodeID, transactionID, latencyMs)
}

// LogPBOSBlockValidation logs the result of a block validation in the pBOS network.
func LogPBOSBlockValidation(nodeID string, blockID string, isValid bool, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBOS_block_validation - Node:%s (blockID:%s, isValid:%t) - Latency:%dms", timestamp, nodeID, blockID, isValid, latencyMs)
}

// LogPBOSFinalityConfirmation logs when block finality is confirmed in the pBOS network.
func LogPBOSFinalityConfirmation(nodeID string, blockID string, latency time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	latencyMs := latency.Milliseconds()
	utils.LogInfo("[%s] VALIDATOR_NODE: PBOS_finality_confirmation - Node:%s (blockID:%s) - Latency:%dms", timestamp, nodeID, blockID, latencyMs)
}
