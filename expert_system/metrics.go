package expert_system

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rulesEvaluatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "rules_evaluated_total",
			Help:      "Total number of rules evaluated.",
		},
		[]string{"rule_id"}, // Partition by rule_id
	)

	rulesMatchedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "rules_matched_total",
			Help:      "Total number of rules matched.",
		},
		[]string{"rule_id"}, // Partition by rule_id that matched
	)

	ruleEvaluationDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "rule_evaluation_duration_seconds",
			Help:      "Histogram of rule evaluation durations.",
			Buckets:   prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"rule_id"},
	)

	activeRulesCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "active_rules_count",
			Help:      "Current number of active rules loaded in the engine.",
		},
	)

	regexCacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "regex_cache_size_items",
			Help:      "Current number of items in the compiled regex cache.",
		},
	)

	regexCacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "regex_cache_hits_total",
			Help:      "Total number of hits in the regex cache.",
		},
	)

	regexCacheMissesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "regex_cache_misses_total",
			Help:      "Total number of misses in the regex cache (led to compilation).",
		},
	)

	ruleLoadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "expert_system",
			Subsystem: "rule_engine",
			Name:      "rule_loads_total",
			Help:      "Total number of rule loading attempts (initial load or reload).",
		},
		[]string{"status"}, // "success" or "failure"
	)
)

// Note: An HTTP endpoint to expose these metrics (e.g., /metrics) would typically be set up
// in the main application that uses this expert_system module, using promhttp.Handler().
// Example:
// http.Handle("/metrics", promhttp.Handler())
// http.ListenAndServe(":9090", nil)
// This part is outside the scope of the expert_system module itself.
