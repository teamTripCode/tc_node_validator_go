package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Peer Discovery Metrics
	DHTPeersDiscovered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "dht_peers_discovered_total",
			Help:      "Total number of peers discovered via DHT, labeled by type (e.g., validator, regular).",
		},
		[]string{"type"},
	)

	IPScanAttempts = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "ip_scan_attempts_total",
			Help:      "Total number of IP scan attempts made.",
		},
	)

	IPScanNodesFound = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "ip_scan_nodes_found_total",
			Help:      "Total number of nodes found via IP scanning, labeled by type.",
		},
		[]string{"type"},
	)

	// Validator Management Metrics
	KnownValidatorsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "known_validators_count",
			Help:      "Current number of known validators in the local list.",
		},
	)

	ValidatorVerifications = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "validator_verifications_total",
			Help:      "Total validator eligibility verification attempts.",
		},
		[]string{"source", "outcome"}, // source: dht, gossip, heartbeat, ip_scan; outcome: eligible, ineligible, error
	)

	ValidatorsAdded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "validators_added_total",
			Help:      "Total number of validators added to the known list, labeled by discovery source.",
		},
		[]string{"source"},
	)

	ValidatorsRemoved = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "validators_removed_total",
			Help:      "Total number of validators removed from the known list, labeled by reason.",
		},
		[]string{"reason"}, // reason: ineligible_recheck, gossip_remove
	)

	// Gossip Protocol Metrics
	GossipMessagesSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "gossip_messages_sent_total",
			Help:      "Total number of gossip messages sent, labeled by event type.",
		},
		[]string{"event_type"}, // ADD_VALIDATOR, REMOVE_VALIDATOR, FULL_SYNC
	)

	GossipMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "gossip_messages_received_total",
			Help:      "Total number of gossip messages received, labeled by event type.",
		},
		[]string{"event_type"},
	)

	// Heartbeat Metrics
	HeartbeatsSent = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "heartbeats_sent_total",
			Help:      "Total number of heartbeats sent.",
		},
	)
	HeartbeatsReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "heartbeats_received_total",
			Help:      "Total number of heartbeats received.",
		},
	)
	HeartbeatFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "heartbeat_failures_total",
			Help:      "Total number of heartbeat failures, labeled by target address.",
		},
		[]string{"target_address"},
	)


	// Reputation System Metrics
	ReputationScoreHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "reputation_scores_histogram",
			Help:      "Distribution of peer reputation scores.",
			Buckets:   prometheus.LinearBuckets(0, 10, 10), // 10 buckets from 0 to 100
		},
	)

	PenalizedPeersGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "penalized_peers_count",
			Help:      "Current number of temporarily penalized peers.",
		},
	)

	// Network Partition Detection Metrics
	NetworkPartitionsDetected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "network_partitions_detected_total",
			Help:      "Total number of times a network partition has been detected.",
		},
	)

	InPartitionStateGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "in_partition_state",
			Help:      "Indicates if the node currently believes it is in a partitioned state (1 if true, 0 if false).",
		},
	)

	ReconnectionAttempts = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "tripcodechain",
			Subsystem: "p2p",
			Name:      "reconnection_attempts_total",
			Help:      "Total number of reconnection attempts triggered by partition detection.",
		},
	)
)
