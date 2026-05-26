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

func TestPolicyEnforcingMemory_Add_EmitsEventWritePolicyDecidedOnAccept(t *testing.T) {
	rec := &recordingObserver{}
	mgr := newCoreManager(t)
	policy := PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictAccept, Kind: in.Kind, Item: in.Item, Reason: "ok"}
	})
	pem, err := NewPolicyEnforcingMemory(mgr, policy, WithObserver(rec))
	if err != nil {
		t.Fatalf("NewPolicyEnforcingMemory: %v", err)
	}
	if _, err := pem.Add(context.Background(), ProposedWrite{
		Kind: coremem.KindWorking, Item: coremem.MemoryItem{Content: "x"}, Source: SourceUserSaved,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := rec.snapshot()
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 event, got %d: %+v", len(got), got)
	}
	if got[0].Name != EventWritePolicyDecided {
		t.Errorf("event Name = %q, want %q", got[0].Name, EventWritePolicyDecided)
	}
	if v, _ := got[0].Attrs["verdict"].(string); v != string(VerdictAccept) {
		t.Errorf("verdict = %v, want %q", got[0].Attrs["verdict"], VerdictAccept)
	}
	if s, _ := got[0].Attrs["source"].(string); s != string(SourceUserSaved) {
		t.Errorf("source = %v, want %q", got[0].Attrs["source"], SourceUserSaved)
	}
	if r, _ := got[0].Attrs["reason"].(string); r != "ok" {
		t.Errorf("reason = %v, want %q", got[0].Attrs["reason"], "ok")
	}
}

func TestPolicyEnforcingMemory_Add_EmitsEventWritePolicyDecidedOnRedact(t *testing.T) {
	rec := &recordingObserver{}
	mgr := newCoreManager(t)
	policy := PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		redacted := in.Item
		redacted.Content = "[REDACTED]"
		return WritePolicyDecision{Verdict: VerdictRedact, Kind: in.Kind, Item: redacted, Reason: "pii"}
	})
	pem, err := NewPolicyEnforcingMemory(mgr, policy, WithObserver(rec))
	if err != nil {
		t.Fatalf("NewPolicyEnforcingMemory: %v", err)
	}
	if _, err := pem.Add(context.Background(), ProposedWrite{
		Kind: coremem.KindWorking, Item: coremem.MemoryItem{Content: "ssn 123-45-6789"},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := rec.snapshot()
	if len(got) != 1 || got[0].Name != EventWritePolicyDecided {
		t.Fatalf("expected 1 EventWritePolicyDecided event, got %+v", got)
	}
	if v, _ := got[0].Attrs["verdict"].(string); v != string(VerdictRedact) {
		t.Errorf("verdict = %v, want %q", got[0].Attrs["verdict"], VerdictRedact)
	}
}

func TestPolicyEnforcingMemory_Add_EmitsEventWritePolicyDecidedOnReject(t *testing.T) {
	rec := &recordingObserver{}
	mgr := newCoreManager(t)
	policy := PolicyFunc(func(_ context.Context, _ ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictReject, Reason: "policy:no-pii"}
	})
	pem, err := NewPolicyEnforcingMemory(mgr, policy, WithObserver(rec))
	if err != nil {
		t.Fatalf("NewPolicyEnforcingMemory: %v", err)
	}
	_, _ = pem.Add(context.Background(), ProposedWrite{
		Kind: coremem.KindWorking, Item: coremem.MemoryItem{Content: "x"}, Source: SourceAgentInferred,
	})
	got := rec.snapshot()
	if len(got) != 1 || got[0].Name != EventWritePolicyDecided {
		t.Fatalf("expected 1 EventWritePolicyDecided event, got %+v", got)
	}
	if v, _ := got[0].Attrs["verdict"].(string); v != string(VerdictReject) {
		t.Errorf("verdict = %v, want %q", got[0].Attrs["verdict"], VerdictReject)
	}
	if r, _ := got[0].Attrs["reason"].(string); r != "policy:no-pii" {
		t.Errorf("reason = %v, want %q", got[0].Attrs["reason"], "policy:no-pii")
	}
}

func TestPolicyAdapter_Sanitize_AcceptReturnsKeepTrue(t *testing.T) {
	policy := PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictAccept, Kind: in.Kind, Item: in.Item}
	})
	adapter := PolicyAdapter{Policy: policy}
	out, keep, err := adapter.Sanitize(context.Background(), coremem.KindWorking, coremem.MemoryItem{Content: "x"})
	if err != nil {
		t.Fatalf("Sanitize: %v", err)
	}
	if !keep {
		t.Error("keep = false on VerdictAccept")
	}
	if out.Content != "x" {
		t.Errorf("Content = %q, want %q", out.Content, "x")
	}
}

func TestPolicyAdapter_Sanitize_RejectReturnsKeepFalse(t *testing.T) {
	policy := PolicyFunc(func(_ context.Context, _ ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictReject}
	})
	adapter := PolicyAdapter{Policy: policy}
	_, keep, err := adapter.Sanitize(context.Background(), coremem.KindWorking, coremem.MemoryItem{Content: "x"})
	if err != nil {
		t.Fatalf("Sanitize: %v", err)
	}
	if keep {
		t.Error("keep = true on VerdictReject")
	}
}

func TestPolicyAdapter_Sanitize_RerouteReturnsKindRerouteUnsupported(t *testing.T) {
	policy := PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
		return WritePolicyDecision{Verdict: VerdictAccept, Kind: coremem.KindEpisodic, Item: in.Item}
	})
	adapter := PolicyAdapter{Policy: policy}
	_, _, err := adapter.Sanitize(context.Background(), coremem.KindWorking, coremem.MemoryItem{Content: "x"})
	if !errors.Is(err, ErrPolicyKindRerouteUnsupported) {
		t.Errorf("err = %v, want errors.Is ErrPolicyKindRerouteUnsupported", err)
	}
}

func TestPolicyEnforcingMemory_CoversAllFourDocumentedDecisions(t *testing.T) {
	// Per docs/memory-roadmap.zh-CN.md §4.3 C-1 exit criterion: the
	// policy interface must cover user-saved, agent-inferred, reject,
	// and redact decisions. One mgr per case so writes don't bleed
	// between assertions.
	cases := []struct {
		name        string
		source      WriteSource
		decide      func(ProposedWrite) WritePolicyDecision
		wantErr     error // nil for success; ErrRejectedByPolicy for reject
		wantLanded  bool  // whether to check that an item exists in mgr after Add
		wantInKind  coremem.Kind
		wantContent string // exact content to expect in the landed item; "" for reject
	}{
		{
			name:        "user-saved direct to episodic",
			source:      SourceUserSaved,
			decide:      func(in ProposedWrite) WritePolicyDecision { return WritePolicyDecision{Verdict: VerdictAccept, Kind: coremem.KindEpisodic, Item: in.Item, Reason: "user-saved-promote"} },
			wantLanded:  true,
			wantInKind:  coremem.KindEpisodic,
			wantContent: "user typed this",
		},
		{
			name:        "agent-inferred routes to working",
			source:      SourceAgentInferred,
			decide:      func(in ProposedWrite) WritePolicyDecision { return WritePolicyDecision{Verdict: VerdictAccept, Kind: coremem.KindWorking, Item: in.Item, Reason: "agent-inferred-defer"} },
			wantLanded:  true,
			wantInKind:  coremem.KindWorking,
			wantContent: "agent inferred this",
		},
		{
			name:    "reject pii",
			source:  SourceAgentInferred,
			decide:  func(_ ProposedWrite) WritePolicyDecision { return WritePolicyDecision{Verdict: VerdictReject, Reason: "policy:pii"} },
			wantErr: ErrRejectedByPolicy,
		},
		{
			name:   "redact secret",
			source: SourceUserSaved,
			decide: func(in ProposedWrite) WritePolicyDecision {
				it := in.Item
				it.Content = "[REDACTED]"
				return WritePolicyDecision{Verdict: VerdictRedact, Kind: in.Kind, Item: it, Reason: "policy:secret"}
			},
			wantLanded:  true,
			wantInKind:  coremem.KindWorking,
			wantContent: "[REDACTED]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := newCoreManager(t)
			pem, err := NewPolicyEnforcingMemory(mgr, PolicyFunc(func(_ context.Context, in ProposedWrite) WritePolicyDecision {
				return tc.decide(in)
			}))
			if err != nil {
				t.Fatalf("NewPolicyEnforcingMemory: %v", err)
			}

			content := tc.wantContent
			if content == "" {
				content = "blocked-content"
			}
			id, err := pem.Add(context.Background(), ProposedWrite{
				Kind:   coremem.KindWorking,
				Item:   coremem.MemoryItem{Content: content},
				Source: tc.source,
			})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want errors.Is %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add: %v", err)
			}
			if tc.wantLanded {
				got, err := mgr.Get(context.Background(), tc.wantInKind, id)
				if err != nil {
					t.Fatalf("Get (kind=%v): %v", tc.wantInKind, err)
				}
				if got.Content != tc.wantContent {
					t.Errorf("Content = %q, want %q", got.Content, tc.wantContent)
				}
			}
		})
	}
}
