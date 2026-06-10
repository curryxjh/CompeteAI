package metrics

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// 蓝图 §13.2 核心指标（进程内计数，/metrics JSON 暴露）。
var (
	TaskCreatedTotal     atomic.Int64
	TaskCompletedTotal   atomic.Int64
	TaskFailedTotal      atomic.Int64
	TaskRecoveryTotal    atomic.Int64
	MessageConsumedTotal atomic.Int64
	MessageRetryTotal    atomic.Int64
	MessageDLQTotal      atomic.Int64
	AgentRunFailTotal    atomic.Int64
)

type agentDuration struct {
	count atomic.Int64
	sumMs atomic.Int64
}

var agentDurations = map[string]*agentDuration{}

func init() {
	for _, a := range []string{"coordinator", "collector", "analyst", "writer", "qa"} {
		agentDurations[a] = &agentDuration{}
	}
}

func IncTaskCreated()     { TaskCreatedTotal.Add(1) }
func IncTaskCompleted()   { TaskCompletedTotal.Add(1) }
func IncTaskFailed()      { TaskFailedTotal.Add(1) }
func IncRecovery()        { TaskRecoveryTotal.Add(1) }
func IncMessageConsumed() { MessageConsumedTotal.Add(1) }
func IncMessageRetry()    { MessageRetryTotal.Add(1) }
func IncMessageDLQ()      { MessageDLQTotal.Add(1) }
func IncAgentFail()       { AgentRunFailTotal.Add(1) }

func RecordAgentRun(agent string, d time.Duration, err error) {
	if ad, ok := agentDurations[agent]; ok {
		ad.count.Add(1)
		ad.sumMs.Add(d.Milliseconds())
	}
	if err != nil {
		IncAgentFail()
	}
}

func Snapshot() map[string]any {
	agentStats := map[string]any{}
	for name, ad := range agentDurations {
		c := ad.count.Load()
		avg := int64(0)
		if c > 0 {
			avg = ad.sumMs.Load() / c
		}
		agentStats[name] = map[string]any{"count": c, "avg_duration_ms": avg}
	}
	return map[string]any{
		"task_created_total":     TaskCreatedTotal.Load(),
		"task_completed_total":   TaskCompletedTotal.Load(),
		"task_failed_total":      TaskFailedTotal.Load(),
		"task_recovery_total":    TaskRecoveryTotal.Load(),
		"message_consumed_total": MessageConsumedTotal.Load(),
		"message_retry_total":    MessageRetryTotal.Load(),
		"message_dlq_total":      MessageDLQTotal.Load(),
		"agent_run_fail_total":   AgentRunFailTotal.Load(),
		"agent_run_duration_ms":  agentStats,
	}
}

func HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Snapshot())
	}
}
