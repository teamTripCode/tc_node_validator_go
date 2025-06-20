package p2p_test // Use _test package to avoid import cycles if p2p types are complex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"tripcodechain_go/consensus"
	"tripcodechain_go/p2p" // Actual package
	"tripcodechain_go/utils"

	"github.com/gorilla/mux"
)

// MockDPoSService is a mock implementation of the DPoS consensus parts needed for tests.
type MockDPoSService struct {
	UpdateValidatorsFunc func(validators []consensus.ValidatorInfo)
	ValidatorsUpdated    []consensus.ValidatorInfo
}

// UpdateValidators records the validators passed to it.
func (m *MockDPoSService) UpdateValidators(validators []consensus.ValidatorInfo) {
	if m.UpdateValidatorsFunc != nil {
		m.UpdateValidatorsFunc(validators)
	}
	m.ValidatorsUpdated = validators
}

// Ensure MockDPoSService implements the DPoS interface expected by Server.
// This is a compile-time check. The p2p.Server.DposService expects *consensus.DPoS.
// For this test, we are directly setting the field DposService in p2p.Server
// to our mock. This works as long as the handler only calls methods present in MockDPoSService.
// If p2p.Server expected an interface type that MockDPoSService implements, that would be cleaner.
// var _ consensus.DPoS = (*MockDPoSService)(nil) // This line would fail as MockDPoSService is not a full DPoS

func TestUpdateValidatorsHandler_Success(t *testing.T) {
	// Suppress log output during tests
	originalLogger := utils.GetLogger()
	utils.InitLogger(false, true) // Disable verbose, enable silent for tests
	defer utils.SetLogger(originalLogger) // Restore original logger

	mockDPoS := &MockDPoSService{}

	// Minimal server setup for this handler
	s := &p2p.Server{
		DposService: mockDPoS, // Assign the mock service
	}

	validatorsPayload := []consensus.ValidatorInfo{
		{Address: "localhost:3001", Stake: "100"},
		{Address: "localhost:3002", Stake: "200"},
	}
	payloadBytes, err := json.Marshal(validatorsPayload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", "/validators/update", bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	// The handler is a method on p2p.Server, so we use s.UpdateValidatorsHandler
	router.HandleFunc("/validators/update", s.UpdateValidatorsHandler).Methods("POST")

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if mockDPoS.ValidatorsUpdated == nil {
		t.Errorf("UpdateValidators was not called on DPoS service")
	} else if len(mockDPoS.ValidatorsUpdated) != 2 {
		t.Errorf("UpdateValidators called with wrong number of validators: got %d want %d", len(mockDPoS.ValidatorsUpdated), 2)
	} else {
		if mockDPoS.ValidatorsUpdated[0].Address != "localhost:3001" {
			t.Errorf("First validator address incorrect: got %s want %s", mockDPoS.ValidatorsUpdated[0].Address, "localhost:3001")
		}
	}

	expectedResp := fmt.Sprintf("Successfully updated validator list with %d validators.", len(validatorsPayload))
	if rr.Body.String() != expectedResp {
		t.Errorf("handler returned unexpected body: got %q want %q", rr.Body.String(), expectedResp)
	}
}

func TestUpdateValidatorsHandler_MalformedJSON(t *testing.T) {
	originalLogger := utils.GetLogger()
	utils.InitLogger(false, true)
	defer utils.SetLogger(originalLogger)

	mockDPoS := &MockDPoSService{}
	s := &p2p.Server{
		DposService: mockDPoS,
	}

	malformedPayload := []byte(`{"address": "localhost:3001", "stake": "100"`) // Missing closing brace

	req, err := http.NewRequest("POST", "/validators/update", bytes.NewBuffer(malformedPayload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/validators/update", s.UpdateValidatorsHandler).Methods("POST")

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expectedErrorMsg := "Invalid validator list format: unexpected EOF" // Error message might vary slightly
	if !bytes.Contains(rr.Body.Bytes(), []byte("Invalid validator list format")) {
		t.Errorf("handler returned unexpected body: got %q, expected to contain %q", rr.Body.String(), expectedErrorMsg)
	}

	if mockDPoS.ValidatorsUpdated != nil {
		t.Errorf("UpdateValidators should not have been called on DPoS service for malformed JSON")
	}
}

func TestUpdateValidatorsHandler_EmptyList(t *testing.T) {
	originalLogger := utils.GetLogger()
	utils.InitLogger(false, true)
	defer utils.SetLogger(originalLogger)

	mockDPoS := &MockDPoSService{}
	s := &p2p.Server{
		DposService: mockDPoS,
	}

	validatorsPayload := []consensus.ValidatorInfo{} // Empty list
	payloadBytes, _ := json.Marshal(validatorsPayload)

	req, err := http.NewRequest("POST", "/validators/update", bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/validators/update", s.UpdateValidatorsHandler).Methods("POST")

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK { // Current behavior is to accept empty list
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check if UpdateValidators was called with an empty list
	if mockDPoS.ValidatorsUpdated == nil { // It should be called, even with an empty list
		t.Errorf("UpdateValidators was not called on DPoS service for empty list")
	} else if len(mockDPoS.ValidatorsUpdated) != 0 {
		t.Errorf("UpdateValidators called with non-empty list: got %d want %d", len(mockDPoS.ValidatorsUpdated), 0)
	}

	expectedResp := `Successfully updated validator list with 0 validators.`
	if rr.Body.String() != expectedResp {
		 t.Errorf("handler returned unexpected body for empty list: got %q want %q", rr.Body.String(), expectedResp)
	}
}

func TestUpdateValidatorsHandler_DPoSUnavailable(t *testing.T) {
	originalLogger := utils.GetLogger()
	utils.InitLogger(false, true)
	defer utils.SetLogger(originalLogger)

	// DposService is nil in the server
	s := &p2p.Server{
		DposService: nil,
	}

	validatorsPayload := []consensus.ValidatorInfo{
		{Address: "localhost:3001", Stake: "100"},
	}
	payloadBytes, _ := json.Marshal(validatorsPayload)

	req, err := http.NewRequest("POST", "/validators/update", bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/validators/update", s.UpdateValidatorsHandler).Methods("POST")

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
	}

	expectedErrorMsg := "Internal server error: DPoS service not available"
	if !bytes.Contains(rr.Body.Bytes(), []byte(expectedErrorMsg)) {
		t.Errorf("handler returned unexpected body: got %q, expected to contain %q", rr.Body.String(), expectedErrorMsg)
	}
}

func TestUpdateValidatorsHandler_Success_WithActualP2PServer(t *testing.T) {
	// More integrated test, still using mock DPoS
	// This test ensures that the p2p.Server can be instantiated as expected by NewServer
	// for the purpose of routing, though we still mock DPoS.

	originalLogger := utils.GetLogger()
	utils.InitLogger(false, true) // Disable verbose, enable silent for tests
	defer utils.SetLogger(originalLogger) // Restore original logger

	mockDPoS := &MockDPoSService{}

	// For a more complete p2p.Server, we might need dummy Node, Chains, Mempools
	// However, NewServer also sets up routes. We need a router.
	// The simplest p2p.Server has a Router field.

	// Let's use a simplified server construction for routing,
	// similar to how it's done in the other tests.
	// The key is that s.UpdateValidatorsHandler is available and s.DposService is our mock.
	 srv := &p2p.Server{
		Router: mux.NewRouter(), // Initialize router to avoid nil panic if any setupRoutes were called by mistake
		DposService: mockDPoS,
	 }
	 // Manually register the handler like in other tests, or ensure NewServer does it.
	 // If NewServer is complex, direct registration is easier for unit test.
	 srv.Router.HandleFunc("/validators/update", srv.UpdateValidatorsHandler).Methods("POST")


	validatorsPayload := []consensus.ValidatorInfo{
		{Address: "node1", Stake: "10"},
	}
	payloadBytes, _ := json.Marshal(validatorsPayload)

	req, err := http.NewRequest("POST", "/validators/update", bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	// Use the server's router if it's configured by NewServer or manually
	srv.Router.ServeHTTP(rr, req)


	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if mockDPoS.ValidatorsUpdated == nil || len(mockDPoS.ValidatorsUpdated) != 1 {
		t.Errorf("UpdateValidators not called correctly")
	}
}
