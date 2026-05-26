package memory

import (
	"context"
	"testing"

	coremem "github.com/costa92/llm-agent/memory"
)

// TestWritePolicy_InterfaceSurface_Compiles is a compile-time
// assertion. If the types in this file go missing or change shape,
// this test will fail to compile, which is the exact signal we want.
func TestWritePolicy_InterfaceSurface_Compiles(t *testing.T) {
	var _ WritePolicy = PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{
			Verdict: VerdictAccept,
			Kind:    in.Kind,
			Item:    in.Item,
		}
	})

	// All three verdicts must be distinct constants.
	if VerdictAccept == VerdictRedact || VerdictAccept == VerdictReject || VerdictRedact == VerdictReject {
		t.Errorf("verdicts must be distinct: accept=%v redact=%v reject=%v",
			VerdictAccept, VerdictRedact, VerdictReject)
	}

	// All three sources must be distinct.
	if SourceUserSaved == SourceAgentInferred || SourceUserSaved == SourceSystem || SourceAgentInferred == SourceSystem {
		t.Errorf("sources must be distinct: user=%v agent=%v system=%v",
			SourceUserSaved, SourceAgentInferred, SourceSystem)
	}

	// ProposedWrite and WritePolicyDecision must accept the documented field set.
	in := ProposedWrite{
		Kind:   coremem.KindWorking,
		Item:   coremem.MemoryItem{Content: "x"},
		Source: SourceUserSaved,
		Hint:   map[string]any{"channel": "chat"},
	}
	out := WritePolicyDecision{
		Verdict: VerdictAccept,
		Kind:    coremem.KindEpisodic,
		Item:    in.Item,
		Reason:  "promote-user-saved",
	}
	if out.Verdict != VerdictAccept || out.Kind != coremem.KindEpisodic {
		t.Errorf("WritePolicyDecision did not round-trip: %+v", out)
	}
}
