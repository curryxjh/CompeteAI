package domain

import (
	"testing"
)

func TestDefaultReceiver(t *testing.T) {
	cases := []struct {
		msgType MessageType
		want    AgentName
	}{
		{MsgTaskCreated, AgentCoordinator},
		{MsgPlanReady, AgentCollector},
		{MsgMaterialsReady, AgentAnalyst},
		{MsgAnalysisReady, AgentWriter},
		{MsgReportReady, AgentQA},
		{MsgQAPass, AgentName("api")},
	}
	for _, c := range cases {
		if got := DefaultReceiver(c.msgType, nil); got != c.want {
			t.Fatalf("DefaultReceiver(%s) = %s, want %s", c.msgType, got, c.want)
		}
	}
}

func TestReworkTarget(t *testing.T) {
	payload := QAResultPayload{TargetAgent: "writer"}
	if got := ReworkTarget(payload); got != AgentWriter {
		t.Fatalf("ReworkTarget = %s, want writer", got)
	}
}

func TestNewEnvelopePayload(t *testing.T) {
	env := NewEnvelope("t1", "tr1", "coordinator", "collector", MsgPlanReady, PlanPayload{
		Summary: "test plan",
	})
	var p PlanPayload
	if err := env.DecodePayload(&p); err != nil {
		t.Fatal(err)
	}
	if p.Summary != "test plan" {
		t.Fatalf("payload summary = %q", p.Summary)
	}
}

func TestRouteNextTopics(t *testing.T) {
	if dec := RouteNext(MsgMaterialsReady, nil); dec.Topic != "analyst.input" {
		t.Fatalf("topic = %s", dec.Topic)
	}
	if dec := RouteNext(MsgQAReject, QAResultPayload{TargetAgent: "analyst"}); dec.ToAgent != AgentAnalyst {
		t.Fatalf("toAgent = %s", dec.ToAgent)
	}
}