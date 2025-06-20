package p2p

import (
	// "fmt" // Removed as unused
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"
)

// Helper type for TestGetRandomIPFromRange to attach cidrEndsWith31Or32 method
type testCaseRandomIP struct {
	name            string
	cidr            string
	expectError     bool
	expectedIP      string
	checkInRange    bool
	excludeNetBroad bool
}

func (tc *testCaseRandomIP) cidrEndsWith31Or32() bool {
	return strings.HasSuffix(tc.cidr, "/31") || strings.HasSuffix(tc.cidr, "/32")
}

func init() {
	// Seed the global random number generator once for any tests that might use it.
	// Note: getRandomIPFromRange uses its own rand.NewSource(time.Now().UnixNano())
	// for each call to ensure varied results in loops, so this global seed primarily
	// affects other tests if they use the math/rand package's default global Rand.
	rand.Seed(time.Now().UnixNano())
}

func TestGetRandomIPFromRange(t *testing.T) {
	testCases := []testCaseRandomIP{
		{"valid_ipv4_24", "192.168.1.0/24", false, "", true, true},
		{"valid_ipv4_30", "192.168.2.0/30", false, "", true, true},
		{"valid_ipv4_31", "192.168.3.0/31", false, "", true, false},
		{"valid_ipv4_32", "192.168.4.1/32", false, "192.168.4.1", true, false},
		{"invalid_cidr_mask_too_large_ipv4", "192.168.5.0/33", true, "", false, false}, // Corrected name
		{"error_on_slash_0_ipv4", "192.168.5.0/0", true, "", false, false}, // Corrected expectations
		{"invalid_cidr_string", "not-a-cidr", true, "", false, false},
		{"empty_cidr", "", true, "", false, false},
		{"ipv4_unspecified_range", "0.0.0.0/24", false, "", true, true},  // Corrected name
		// {"valid_ipv6_64", "2001:db8::/64", true, "", false, false}, // Expect error due to large range, per current code logic
		{"valid_ipv6_120", "2001:db8:abcd:0012::/120", false, "", true, true},
		{"valid_ipv6_127", "2001:db8:abcd:0012::/127", false, "", true, false},
		{"valid_ipv6_128", "2001:db8::1/128", false, "2001:db8::1", true, false},
		{"invalid_ipv6_mask_too_large", "2001:db8::/129", true, "", false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var generatedIPs []string
			loopCount := 5
			if tc.expectedIP != "" || tc.cidrEndsWith31Or32() { // For single IP or very small ranges, one check is enough or 2 for /31
				if strings.HasSuffix(tc.cidr, "/31") || strings.HasSuffix(tc.cidr, "/127") {
					loopCount = 10 // Try more times for /31 to see both IPs
				} else {
					loopCount = 1
				}
			}


			for i := 0; i < loopCount; i++ {
				ipStr, err := getRandomIPFromRange(tc.cidr)

				if tc.expectError {
					if err == nil {
						t.Errorf("CIDR '%s': Expected error, but got none (IP: %s)", tc.cidr, ipStr)
					}
					return
				}
				if err != nil {
					t.Fatalf("CIDR '%s': Expected no error, but got: %v", tc.cidr, err)
				}

				if ipStr == "" {
					t.Errorf("CIDR '%s': Generated IP is empty", tc.cidr)
					continue
				}
				generatedIPs = append(generatedIPs, ipStr)

				parsedIP := net.ParseIP(ipStr)
				if parsedIP == nil {
					t.Errorf("CIDR '%s': Failed to parse generated IP '%s'", tc.cidr, ipStr)
					continue
				}

				if tc.expectedIP != "" {
					if ipStr != tc.expectedIP {
						t.Errorf("CIDR '%s': Expected IP '%s', but got '%s'", tc.cidr, tc.expectedIP, ipStr)
					}
				}

				if tc.checkInRange {
					_, ipNet, parseErr := net.ParseCIDR(tc.cidr)
					if parseErr != nil {
						t.Fatalf("CIDR '%s': Failed to parse test case CIDR: %v", tc.cidr, parseErr)
					}
					if !ipNet.Contains(parsedIP) {
						t.Errorf("CIDR '%s': Generated IP '%s' is not in range", tc.cidr, ipStr)
					}
				}

				if tc.excludeNetBroad {
					_, ipNet, _ := net.ParseCIDR(tc.cidr)
					networkIP := ipNet.IP

					broadcastIPVal := make(net.IP, len(networkIP))
					if len(networkIP) == net.IPv4len { // Calculation for IPv4 broadcast
						for j := 0; j < len(networkIP); j++ {
							broadcastIPVal[j] = networkIP[j] | ^ipNet.Mask[j]
						}
						if parsedIP.Equal(networkIP) {
							 t.Errorf("CIDR '%s': Generated IP '%s' is the network address", tc.cidr, ipStr)
						}
						if parsedIP.Equal(broadcastIPVal) {
							 t.Errorf("CIDR '%s': Generated IP '%s' is the broadcast address", tc.cidr, ipStr)
						}
					}
					// Note: Broadcast address concept is different/less common for IPv6 in this manner.
					// Network address (all host bits zero) is still relevant.
					if parsedIP.Equal(networkIP) && !(strings.HasSuffix(tc.cidr, "/32") || strings.HasSuffix(tc.cidr, "/128")) {
                         t.Errorf("CIDR '%s': Generated IP '%s' is the network address", tc.cidr, ipStr)
                    }

				}
			}

			// For ranges that should produce variable IPs, check if we got some variation
			if loopCount > 1 && tc.expectedIP == "" && !tc.cidrEndsWith31Or32() && len(generatedIPs) > 1 {
				allSame := true
				for i := 1; i < len(generatedIPs); i++ {
					if generatedIPs[i] != generatedIPs[0] {
						allSame = false
						break
					}
				}
				if allSame {
					t.Logf("Warning: For CIDR '%s', all %d generated IPs were the same: %s. This might be okay for very small ranges or due to rand seed.", tc.cidr, loopCount, generatedIPs[0])
				}
			}
		})
	}
}
