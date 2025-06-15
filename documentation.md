Command to run
```bash
go run main.go -port=3000 -verbose=true -datadir=data -seed=localhost:3001
```

---

## MCP and Distributed LLM Service Configuration

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