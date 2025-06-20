Command to run
```bash
go run main.go -port=3000 -verbose=true -datadir=data -seed=localhost:3001
```

---

## Table of Contents
1.  [P2P Network Features](#p2p-network-features)
    *   [Peer Discovery and Synchronization](#peer-discovery-and-synchronization)
    *   [Validator Eligibility and Management](#validator-eligibility-and-management)
    *   [Network Resilience](#network-resilience)
    *   [Monitoring](#monitoring)
    *   [Key Configuration Variables](#key-configuration-variables)
2.  [MCP and Distributed LLM Service Configuration](#mcp-and-distributed-llm-service-configuration)
    *   [MCP ConfigMap (`mcp-config`)](#mcp-configmap-mcp-config)
    *   [Environment Variables (MCP/LLM)](#environment-variables-mcpllm)
    *   [Distributed LLM Service API](#distributed-llm-service-api)
3.  [Unit Test Candidates](#unit-test-candidates)


## 1. P2P Network Features

This section describes the peer-to-peer networking capabilities implemented in the Tripcodechain node.

### Peer Discovery and Synchronization

Nodes employ multiple mechanisms to discover each other and synchronize validator lists.

*   **DHT-based Discovery (LibP2P Kademlia):**
    *   **Purpose:** Allows nodes to find each other in a decentralized manner. Validators, in particular, advertise their presence under a specific service tag.
    *   **Mechanism:** Each node initializes a Kademlia DHT instance. Validator nodes advertise themselves using `discovery.Advertise` under the `ValidatorServiceTag` (e.g., `"tripcodechain-validator"`). Non-validator nodes (or any node seeking peers) can then use `routingDiscovery.FindPeers` to locate these advertised validators.
    *   **Bootstrap:** Nodes connect to a list of pre-configured bootstrap peers (LibP2P multiaddresses) to join the DHT network. This is facilitated by `Node.BootstrapDHT()`. HTTP seed nodes provided at startup can also lead to DHT bootstrapping if those seeds provide LibP2P address information or are part of the `initialBootstrapPeers` list.

*   **Gossip Protocol (LibP2P PubSub):**
    *   **Purpose:** Used for propagating lists of known, verified validators and for immediate notification of validator additions or removals.
    *   **Mechanism:** Nodes join a specific PubSub topic (`GossipSubValidatorTopic`).
        *   **Event-Driven Updates:** When a node adds a newly verified validator (e.g., discovered via DHT, IP scan, or heartbeat and then verified), it broadcasts an `ADD_VALIDATOR` message containing the new validator's `NodeStatus`. When a validator is removed (e.g., due to failing periodic re-checks), a `REMOVE_VALIDATOR` message is broadcast.
        *   **Periodic Full Sync:** Nodes also periodically broadcast a `FULL_SYNC` message containing their entire list of known and verified validators. This helps ensure eventual consistency and allows new nodes to catch up.
    *   **Message Structure (`ValidatorGossipMessage`):** Includes `SenderNodeID` (HTTP Address of sender), `EventType` (`ADD_VALIDATOR`, `REMOVE_VALIDATOR`, `FULL_SYNC`), and `Validators` (a list of `NodeStatus`, usually one for ADD/REMOVE events).
    *   **Processing:** Upon receiving gossip messages, nodes verify the eligibility of any new validators before adding them to their local `knownNodes` list. `REMOVE_VALIDATOR` events cause the specified validator to be removed.

*   **Enhanced Heartbeats:**
    *   **Purpose:** Regular HTTP-based messages sent between known peers to exchange current validator lists and assess peer liveness/responsiveness for the reputation system.
    *   **Mechanism:** Nodes periodically send a `HeartbeatPayload` to their known peers via a POST request to the `/heartbeat` endpoint. This payload includes the sender's `NodeID` (HTTP Address), `Libp2pPeerID`, and its list of `KnownValidators`.
    *   **Processing:** When a node receives a heartbeat, it updates the `LastSeenOnline` time for the sender in its reputation tracking. It also processes the `KnownValidators` list from the payload: new validators are verified for eligibility and then added to the local `knownNodes` list. This helps in faster dissemination of validator information among directly connected peers.

*   **Random IP Scanning (Optional):**
    *   **Purpose:** Provides a bootstrap mechanism for discovering initial peers/validators in new or isolated networks where DHT bootstrap peers or seed nodes might not be available or sufficient.
    *   **Mechanism:** If enabled, the node scans random IP addresses within configured CIDR ranges (e.g., "192.168.1.0/24"). It attempts to connect to a target HTTP port on these IPs and queries the `/node-status` endpoint.
    *   If a responding node is identified as a "validator" and passes the `VerifyValidatorEligibility` check, it's added to the local `knownNodes` list.
    *   **Configuration:** Enabled by `IP_SCANNER_ENABLED` ENV var. Ranges via `IP_SCAN_RANGES` (comma-separated CIDRs). Target port via `IP_SCAN_TARGET_PORT`.
    *   **Control:** The scanner runs only if the number of known peers is below a certain threshold (e.g., 3) and performs a limited number of scan attempts per cycle (`maxScanAttemptsPerRun`).

### Validator Eligibility and Management

Ensuring that only eligible nodes act as validators is crucial for network security and consensus.

*   **Real-time Stake Validation (`consensus.VerifyValidatorEligibility`):**
    *   **Purpose:** To check if a given node (identified by its HTTP address) is eligible to be a validator.
    *   **Mechanism:** This function, located in the `consensus` package, is called whenever a node needs to confirm a peer's validator status.
        1.  **Registered Validators:** If the `validatorAddress` is already known to the local DPoS instance, it checks its current `Stake` against `blockchain.MinValidatorStake` and its `IsActive` status.
        2.  **Potential Validators:** If the `validatorAddress` is not registered in DPoS, its general balance (from `currency.GetBalance`) is checked against `blockchain.MinValidatorStake`. This determines if the node *could* become a validator by staking.
    *   **Integration:** This function is used during P2P registration (HTTP `/register`), when processing peers from DHT discovery, when processing validators from gossip messages, and when processing validator lists from heartbeats, and during IP scanning.

*   **Periodic Re-checks and Removal:**
    *   **Purpose:** To ensure that validators already in the local `knownNodes` list remain eligible over time.
    *   **Mechanism:** The `StartValidatorMonitoring` function in `p2p.Node` initiates a goroutine (`periodicallyRecheckValidators`) that runs every `ValidatorRecheckInterval` (e.g., 5 minutes).
    *   This goroutine iterates through all known validators and re-verifies their eligibility using `consensus.VerifyValidatorEligibility`.
    *   If a validator is found to be no longer eligible (or an error occurs during the check), it is removed from the local `knownNodes` list. A `REMOVE_VALIDATOR` gossip message is then broadcast to inform peers.

### Network Resilience

Mechanisms are in place to handle unreliable peers and potential network segmentation.

*   **Reputation Scoring System:**
    *   **Purpose:** To track peer behavior and assign a reputation score, allowing the node to prioritize interactions with more reliable peers.
    *   **Mechanism:** Each peer (identified by HTTP address) has a `PeerReputation` struct associated with it, stored in `Node.peerReputations`. This struct tracks:
        *   Connection attempts (successful/failed).
        *   Heartbeat responses (successful/failed).
        *   Network latency (total, observations, average).
        *   `LastSeenOnline` and `LastUpdated` timestamps.
        *   A `ReputationScore` (0-100, starting at a neutral 75.0).
        *   `IsTemporarilyPenalized` flag and `PenaltyEndTime`.
    *   **Score Calculation (`calculateReputationScoreInternal`):** Scores are updated after relevant interactions (e.g., heartbeat responses). The current logic considers connection success rate, heartbeat success rate, and average latency.
    *   **Penalization:** If a peer's score falls below a threshold (e.g., 20) after a minimum number of interactions, it's temporarily penalized (e.g., for 10 minutes). Heartbeats are not sent to penalized peers.
    *   **Rehabilitation:** Penalties are lifted if the score improves above a threshold (e.g., 40) after the penalty duration, or significantly (e.g., above 60) before the duration ends. Penalties can be extended if the score remains low.
    *   **Metric Collection:** Key reputation data points (e.g., successful/failed heartbeats, latency) are recorded via methods like `RecordHeartbeatResponse` and `UpdatePeerLastSeen`.

*   **Network Partition Detection:**
    *   **Purpose:** To detect if the node might be isolated from the main validator set or if there's a significant disagreement on who the validators are.
    *   **Mechanism:**
        1.  **View Collection:** Nodes store the list of validators reported by their peers (via `GossipEventFullSync` messages and `HeartbeatPayload`) in `Node.peerValidatorViews`, keyed by the peer's HTTP address.
        2.  **Periodic Check (`checkForNetworkPartition`):** Runs every `partitionCheckInterval` (e.g., 2 minutes). It compares the local node's current set of known validators against the views received from other peers.
        3.  **Jaccard Index:** The similarity between the local validator set and each peer's set is calculated using the Jaccard index.
        4.  **Detection:** If a significant percentage (e.g., >=50%) of reporting peers show a Jaccard index below a `partitionDetectionThreshold` (e.g., 0.3) or major discrepancies (one set empty, the other not), a potential network partition is flagged (`isPotentiallyPartitioned = true`).
        5.  **Reconnection Strategy (`triggerReconnectionActions`):** When a partition is detected (and debounced using `partitionDebounceDuration`), the node attempts to re-bootstrap its DHT connection using its `defaultBootstrapPeers`.
        6.  **Resolution:** If a partition was previously detected but subsequent checks show no significant discrepancies with enough peers, the state is cleared.

### Monitoring

*   **Prometheus Metrics:**
    *   **Purpose:** To expose internal P2P state and events for monitoring and alerting.
    *   **Mechanism:** The node exposes a `/metrics` HTTP endpoint (typically on the same port as its main P2P HTTP services). This endpoint provides data in Prometheus format.
    *   **Key Metric Groups (Namespaced under `tripcodechain_p2p_`):**
        *   Peer Discovery: `dht_peers_discovered_total`, `ip_scan_attempts_total`, `ip_scan_nodes_found_total`.
        *   Validator Management: `known_validators_count`, `validator_verifications_total`, `validators_added_total`, `validators_removed_total`.
        *   Gossip Protocol: `gossip_messages_sent_total`, `gossip_messages_received_total`.
        *   Heartbeats: `heartbeats_sent_total`, `heartbeats_received_total`, `heartbeat_failures_total`.
        *   Reputation System: `reputation_scores_histogram`, `penalized_peers_count`.
        *   Network Partition: `network_partitions_detected_total`, `in_partition_state`, `reconnection_attempts_total`.
    *   **Updates:** Metrics are updated throughout the P2P logic (e.g., counters incremented on events, gauges set periodically by `StartMetricsUpdater`).

### Key Configuration Variables

These are primarily configured via environment variables (with some command-line flags as fallbacks or alternatives):

*   **Port (flag: `-port`, default: `3001`)**: The main HTTP port for the node's API and P2P communication. LibP2P services typically run on `PORT + 1000`.
*   **Node Type (env: `NODE_TYPE`, flag: `-nodetype`)**: Defines the role of the node (e.g., "validator", "regular"). Influences advertising behavior and other P2P logic.
*   **Seed Nodes (HTTP) (env: `SEED_NODES`, flag: `-seed`)**: Comma-separated list of HTTP addresses (e.g., `localhost:3001,localhost:3002`) of seed nodes for initial peer discovery via HTTP registration.
*   **LibP2P Bootstrap Peers (env: `BOOTSTRAP_PEERS`)**: Comma-separated list of LibP2P multiaddresses for initial DHT bootstrapping. These are stored as `defaultBootstrapPeers` in the node and used for reconnection.
*   **IP Scanner Enabled (env: `IP_SCANNER_ENABLED`)**: Set to "true" to enable the random IP scanner. Defaults to false.
*   **IP Scan Ranges (env: `IP_SCAN_RANGES`)**: Comma-separated list of CIDR ranges for the IP scanner (e.g., "192.168.1.0/24,10.0.0.0/16"). Defaults to "127.0.0.1/24" if scanner is enabled but no ranges are provided.
*   **IP Scan Target Port (env: `IP_SCAN_TARGET_PORT`)**: The HTTP port the IP scanner should attempt to connect to on scanned IPs. Defaults to the node's own HTTP port (`config.Port`).
*   **Data Directory (flag: `-datadir`, default: `"data"`)**: Directory for storing blockchain data.
*   **Verbose Logging (flag: `-verbose`, default: `true`)**: Enables detailed logging.

---
## 3. Unit Test Candidates

This section lists key functions and areas that are good candidates for unit testing to ensure the robustness and correctness of the P2P and consensus functionalities.

### `p2p.getRandomIPFromRange(cidr string) (string, error)`
*   **Valid IPv4 CIDRs:**
    *   Test with common CIDRs like `/24`, `/30`, `/31`, `/32`.
    *   Verify that the generated IP falls within the expected range.
*   **Exclusion of Network/Broadcast:**
    *   For ranges larger than 2 IPs (e.g., `/24`, `/30`), test multiple generations to ensure network and broadcast addresses are generally not selected (probabilistic, but should be rare).
*   **Edge Case CIDRs (IPv4):**
    *   Test `/31` (2 IPs, both should be selectable).
    *   Test `/32` (1 IP, should return that IP).
*   **Valid IPv6 CIDRs (if supported by implementation, focus on IPv4 if complex):**
    *   Test with examples like `/64`, `/127`, `/128`.
*   **Invalid Inputs:**
    *   Test with malformed CIDR strings (e.g., "invalid", "192.168.1/33").
    *   Test with CIDRs that result in impractical host ranges if not already caught (e.g., very large hostBits for IPv4 if error handling for this is present).
*   **Error Handling:** Ensure appropriate errors are returned for invalid inputs.

### `p2p.Node.calculateReputationScoreInternal(rep *PeerReputation)`
*(Note: This method is private. Testing might require a public wrapper or making it temporarily public for tests, or testing its effects through public methods like `RecordHeartbeatResponse`.)*
*   **Neutral Score:** Test with a new `PeerReputation` object (no interaction data); score should be the default neutral score (e.g., 75.0 or 50.0 depending on base).
*   **Connection Success/Failure:**
    *   High success rate for connections, no heartbeats: verify score increases appropriately.
    *   High failure rate for connections, no heartbeats: verify score decreases.
*   **Heartbeat Success/Failure:**
    *   High success rate for heartbeats, no connections: verify score increases.
    *   High failure rate for heartbeats, no connections: verify score decreases.
*   **Latency Impact:**
    *   Test with low average latency: score should not be negatively impacted or might slightly increase.
    *   Test with very high average latency (e.g., > 1s): score should decrease significantly.
    *   Test with moderate average latency (e.g., 300ms): score should decrease moderately.
*   **Combined Metrics:** Test with various combinations of connection, heartbeat, and latency data.
*   **Score Boundaries:** Ensure score stays within 0-100.
*   **Penalization Logic:**
    *   Scenario: Low score + sufficient interactions (`minInteractionsForPenalization`) -> verify `IsTemporarilyPenalized` becomes true and `PenaltyEndTime` is set.
    *   Scenario: Penalized, penalty time passes, score still low -> verify penalty is extended (if implemented) or remains.
    *   Scenario: Penalized, penalty time passes, score improves above rehabilitation threshold -> verify `IsTemporarilyPenalized` becomes false.
    *   Scenario: Penalized, score improves significantly *before* penalty time passes -> verify `IsTemporarilyPenalized` becomes false (proactive rehabilitation).

### `consensus.VerifyValidatorEligibility(dpos *DPoS, validatorAddress string) (bool, error)`
*(Requires mocking `dpos.GetValidatorInfo` and `currency.GetBalance`.)*
*   **Registered, Active, Sufficient Stake:** Expected: `(true, nil)`.
*   **Registered, Active, Insufficient Stake:** Expected: `(false, nil)`.
*   **Registered, Inactive, Sufficient Stake:** Expected: `(false, nil)`.
*   **Unregistered, Sufficient Balance (potential validator):** Expected: `(true, nil)`.
*   **Unregistered, Insufficient Balance:** Expected: `(false, nil)`.
*   **Error from `currency.GetBalance`:** Expected: `(false, error)`.
*   **DPoS instance is nil:** Expected: `(false, error)`.

### Jaccard Index Calculation (within `p2p.Node.checkForNetworkPartition`)
*(Consider extracting the Jaccard index logic into a standalone, testable helper function if it's complex enough.)*
*   **Identical Sets:** Two sets with the same validator addresses. Expected Jaccard Index: 1.0.
*   **Disjoint Sets:** Two sets with no common validator addresses. Expected Jaccard Index: 0.0.
*   **Partially Overlapping Sets:** Test with some common and some unique addresses. Verify correct Jaccard calculation.
*   **Empty Sets:**
    *   Both local and peer sets are empty. Expected: Skip or Jaccard Index 1.0 (if union is 0, handle division by zero).
    *   One set empty, the other not. Expected: Jaccard Index 0.0.
*   **Thresholds:** Test `checkForNetworkPartition`'s decision logic based on Jaccard index results against `partitionDetectionThreshold` and `minPeersForPartitionCheck`.

### `p2p.Node.StorePeerValidatorView`
*   Test adding a view for a new peer.
*   Test updating a view for an existing peer.
*   Test that views from self or with an empty peer address are ignored.
*   Verify correct locking (`partitionCheckMutex`).

### `p2p.Node` - Event-Driven Gossip Triggers
*   **`AddNodeStatus`:**
    *   Test that `publishValidatorEvent(GossipEventAddValidator, ...)` is called when a new node is added with `NodeType="validator"`.
    *   Test that it's called when an existing node's `NodeType` changes to "validator".
    *   Test that it's *not* called if a non-validator node is added/updated, or if an existing validator's status doesn't change its validator role.
*   **`recheckKnownValidators`:**
    *   Test that `publishValidatorEvent(GossipEventRemoveValidator, ...)` is called for each validator that becomes ineligible and is removed.
    *   Verify that the correct `NodeStatus` of the removed validator is passed.

This list provides a good starting point for ensuring the reliability of the new P2P features. Integration tests will also be crucial to verify the interplay between these components.

---
## 2. MCP and Distributed LLM Service Configuration

This section details the configuration options for the Model Context Protocol (MCP) and the Distributed LLM Service.

### MCP ConfigMap (`mcp-config`)

The primary configuration for MCP and the associated LLM services is managed through a Kubernetes ConfigMap named `mcp-config`. This ConfigMap is typically mounted into the validator node pods.

-   **Purpose**: To provide runtime configuration for the MCP server integrated within each node and for the Distributed LLM Service that orchestrates queries across MCP-enabled nodes.
-   **Location**: Defined in `k8s/mcp-configmap.yaml`. This file is deployed to the Kubernetes cluster.
-   **Mounted File**: The `data` section of the ConfigMap contains a key `config.json`. The content of this key is mounted as a file at `/etc/tripcodechain/mcp/config.json` within the validator node's container (this path is specified by the `MCP_CONFIG_PATH` environment variable).

#### Example `config.json` Keys:

```json
{
  "mcpServiceEnabled": true,
  "defaultQueryTimeoutSeconds": 10,
  "maxConcurrentQueriesPerNode": 5,
  "logLevel": "INFO",
  "llmService": {
    "minResponsesForAggregation": 1,
    "responseAggregationStrategy": "firstValid"
  },
  "nodeBlacklist": [
    "node-id-to-blacklist-1",
    "node-id-to-blacklist-2"
  ]
}
```

-   **`mcpServiceEnabled` (boolean)**: Enables or disables the MCP service on the node.
-   **`defaultQueryTimeoutSeconds` (integer)**: Default timeout in seconds for queries initiated by the Distributed LLM Service.
-   **`maxConcurrentQueriesPerNode` (integer)**: Limits how many MCP queries this node will process concurrently from other nodes.
-   **`logLevel` (string)**: Sets the logging level for MCP and LLM service components (e.g., "DEBUG", "INFO", "WARN", "ERROR"). This might be overridden by global log settings.
-   **`llmService.minResponsesForAggregation` (integer)**: The minimum number of responses the Distributed LLM Service should wait for before attempting aggregation (unless timeout occurs first).
-   **`llmService.responseAggregationStrategy` (string)**: The strategy used to aggregate responses from multiple MCP nodes (e.g., "firstValid", "majorityConsensus", "averageNumerical").
-   **`nodeBlacklist` (array of strings)**: A list of MCP Node IDs from which this node will ignore or reject queries.

### Environment Variables

The following environment variables are used by the validator node to configure its MCP and Distributed LLM service components:

-   **`MCP_LOG_LEVEL`**: Overrides the `logLevel` from `config.json` if set. Sets the logging level for MCP components.
    *   Example: `INFO`
-   **`MCP_CONFIG_PATH`**: Specifies the absolute path to the MCP configuration file (mounted from the ConfigMap).
    *   Example: `/etc/tripcodechain/mcp/config.json`

### Distributed LLM Service API

The Distributed LLM Service exposes an API endpoint for initiating queries across the MCP network.

-   **Endpoint**: `POST /api/v1/llm/query`
-   **Request Payload (JSON)**: The client should send a JSON object detailing the query.
    ```json
    {
      "prompt": "Translate the following English text to French: 'Hello, world.'",
      "model": "optional-model-identifier", // Optional: specify a target model or type
      "parameters": { // Optional: any additional parameters for the LLM
        "temperature": 0.7
      }
    }
    ```
    *   `prompt` (string, required): The actual query or prompt to be sent to the LLMs.
    *   `model` (string, optional): A specific model identifier or type that the querying node requests. MCP servers can use this to route to the appropriate local model.
    *   `parameters` (object, optional): Additional parameters to be passed to the underlying LLM.

-   **Response Payload (JSON)**: The API responds with a JSON array of `p2p.MCPResponse` objects collected from the participating MCP nodes, or potentially a single aggregated response if server-side aggregation is implemented for this endpoint.
    ```json
    [
      {
        "queryId": "uuid-of-the-original-query",
        "timestamp": 1678886400,
        "originNodeId": "node-id-of-original-querier",
        "responderNodeId": "node-id-of-this-mcp-responder",
        "payload": "base64-encoded-response-payload-or-json-string", // Actual LLM response
        "signature": {
          "nodeId": "node-id-of-this-mcp-responder",
          "signature": "signature-of-the-response"
        }
      }
      // ... more responses
    ]
    ```
    If an error occurs during the query process (e.g., timeout before any responses, internal error), an appropriate JSON error object will be returned.