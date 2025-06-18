package llm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// LocalLLMClient defines the interface for interacting with a local LLM.
type LocalLLMClient interface {
	QueryLLM(payload []byte) (response []byte, err error)
}

// HTTPLocalLLMClient implements LocalLLMClient for an LLM accessible via HTTP.
type HTTPLocalLLMClient struct {
	llmEndpointURL string
	httpClient     *http.Client
}

// NewHTTPLocalLLMClient creates a new HTTPLocalLLMClient.
func NewHTTPLocalLLMClient(endpointURL string) *HTTPLocalLLMClient {
	return &HTTPLocalLLMClient{
		llmEndpointURL: endpointURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Default timeout for HTTP requests
		},
	}
}

// QueryLLM sends a payload to the local LLM endpoint and returns the response.
// It includes a retry mechanism for transient errors.
func (c *HTTPLocalLLMClient) QueryLLM(payload []byte) ([]byte, error) {
	var lastErr error
	retries := 3

	for i := 0; i < retries; i++ {
		req, err := http.NewRequest("POST", c.llmEndpointURL, bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request for local LLM: %w", err)
		}
		req.Header.Set("Content-Type", "application/json") // Assuming JSON payload

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request to local LLM failed (attempt %d/%d): %w", i+1, retries, err)
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff might be better, simple delay for now
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			lastErr = fmt.Errorf("local LLM returned non-OK status (attempt %d/%d): %d - %s", i+1, retries, resp.StatusCode, string(bodyBytes))
			// Only retry on 5xx server errors, not on 4xx client errors
			if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				time.Sleep(time.Duration(i+1) * time.Second)
				continue
			}
			return nil, lastErr // Non-retryable error
		}

		responseBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body from local LLM (attempt %d/%d): %w", i+1, retries, err)
			// This could be a network issue during response reading, so retry
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return responseBody, nil // Success
	}

	return nil, fmt.Errorf("failed to query local LLM after %d retries: %w", retries, lastErr)
}
