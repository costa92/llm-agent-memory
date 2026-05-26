package memory

import (
	"context"
	"errors"
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

func TestPolicyEnforcingMemory_Add_VerdictAccept_WritesToDecidedKind(t *testing.T) {
	mgr := newCoreManager(t)
	policy := PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		// Reroute: user-saved memories go to Episodic regardless of input kind.
		return WritePolicyDecision{
			Verdict: VerdictAccept,
			Kind:    coremem.KindEpisodic,
			Item:    in.Item,
			Reason:  "promote-user-saved",
		}
	})
	pem, err := NewPolicyEnforcingMemory(mgr, policy)
	if err != nil {
		t.Fatalf("NewPolicyEnforcingMemory: %v", err)
	}

	ctx := context.Background()
	id, err := pem.Add(ctx, ProposedWrite{
		Kind:   coremem.KindWorking, // caller asked for Working...
		Item:   coremem.MemoryItem{Content: "remember me"},
		Source: SourceUserSaved,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id == "" {
		t.Fatal("Add returned empty id")
	}

	// Confirm the item landed in Episodic (the decided kind), not Working.
	got, err := mgr.Get(ctx, coremem.KindEpisodic, id)
	if err != nil {
		t.Fatalf("Get from Episodic: %v", err)
	}
	if got.Content != "remember me" {
		t.Errorf("Episodic item Content = %q, want %q", got.Content, "remember me")
	}
}

func TestPolicyEnforcingMemory_Add_VerdictReject_ReturnsErrRejectedByPolicy(t *testing.T) {
	mgr := newCoreManager(t)
	policy := PolicyFunc(func(_ context.Context, _ ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictReject, Reason: "test-reject"}
	})
	pem, err := NewPolicyEnforcingMemory(mgr, policy)
	if err != nil {
		t.Fatalf("NewPolicyEnforcingMemory: %v", err)
	}

	_, err = pem.Add(context.Background(), ProposedWrite{
		Kind: coremem.KindWorking,
		Item: coremem.MemoryItem{Content: "blocked"},
	})
	if err == nil {
		t.Fatal("Add returned nil error on VerdictReject")
	}
	if !errors.Is(err, ErrRejectedByPolicy) {
		t.Errorf("Add err = %v, want errors.Is ErrRejectedByPolicy", err)
	}
}
