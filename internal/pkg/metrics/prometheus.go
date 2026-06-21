package metrics

import (
	"fmt"
	"net/http"
	"strings"
)

// PrometheusHandler 暴露 Prometheus 文本格式（蓝图 §13.2）。
func PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s := Snapshot()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var b strings.Builder
		writeCounter(&b, "task_created_total", s["task_created_total"])
		writeCounter(&b, "task_completed_total", s["task_completed_total"])
		writeCounter(&b, "task_failed_total", s["task_failed_total"])
		writeCounter(&b, "task_recovery_total", s["task_recovery_total"])
		writeCounter(&b, "message_consumed_total", s["message_consumed_total"])
		writeCounter(&b, "message_retry_total", s["message_retry_total"])
		writeCounter(&b, "message_dlq_total", s["message_dlq_total"])
		writeCounter(&b, "agent_run_fail_total", s["agent_run_fail_total"])
		_, _ = w.Write([]byte(b.String()))
	}
}

func writeCounter(b *strings.Builder, name string, v any) {
	switch n := v.(type) {
	case int64:
		fmt.Fprintf(b, "# TYPE %s counter\n%s %d\n", name, name, n)
	}
}
