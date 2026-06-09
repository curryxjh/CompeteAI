package memory

import (
	"sync/atomic"
	"time"
)

var (
	buildContextLatencyMs atomic.Int64
	ingestLatencyMs       atomic.Int64
	injectedTokens        atomic.Int64
	invalidatedFacts      atomic.Int64
)

func recordBuildLatency(d time.Duration) {
	buildContextLatencyMs.Store(d.Milliseconds())
}

func recordIngestLatency(d time.Duration) {
	ingestLatencyMs.Store(d.Milliseconds())
}

func recordInjectedTokens(n int) {
	injectedTokens.Add(int64(n))
}

func recordInvalidated() {
	invalidatedFacts.Add(1)
}

// MetricsSnapshot 可观测性指标（§19）。
func MetricsSnapshot() map[string]any {
	return map[string]any{
		"memory_build_context_latency_ms": buildContextLatencyMs.Load(),
		"memory_ingest_latency_ms":        ingestLatencyMs.Load(),
		"memory_prompt_token_injected":    injectedTokens.Load(),
		"memory_invalidated_fact_count":   invalidatedFacts.Load(),
	}
}
