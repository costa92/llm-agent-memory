package memory

import contractmem "github.com/costa92/llm-agent-contract/memory"

// The eight metadata helpers (Get/Set Source + Category, Is/Set Pinned
// + Disabled) and the Source classification constants now live in the
// contract leaf github.com/costa92/llm-agent-contract/memory. They are
// re-exported here so existing in-package callers keep using the bare
// names with identical behavior (the contract impl writes the same
// reserved "_"-prefixed metadata keys this package always used).

const (
	SourceUserSaved     = contractmem.SourceUserSaved
	SourceAgentInferred = contractmem.SourceAgentInferred
	SourceSystem        = contractmem.SourceSystem
	SourceUnknown       = contractmem.SourceUnknown
)

// GetSource / SetSource / GetCategory / SetCategory / IsPinned /
// SetPinned / IsDisabled / SetDisabled forward to the contract helpers
// so there is exactly one behavior across the stack.
func GetSource(it MemoryItem) Source            { return contractmem.GetSource(it) }
func SetSource(it *MemoryItem, src Source)      { contractmem.SetSource(it, src) }
func GetCategory(it MemoryItem) Category        { return contractmem.GetCategory(it) }
func SetCategory(it *MemoryItem, cat Category)  { contractmem.SetCategory(it, cat) }
func IsPinned(it MemoryItem) bool               { return contractmem.IsPinned(it) }
func SetPinned(it *MemoryItem, pinned bool)     { contractmem.SetPinned(it, pinned) }
func IsDisabled(it MemoryItem) bool             { return contractmem.IsDisabled(it) }
func SetDisabled(it *MemoryItem, disabled bool) { contractmem.SetDisabled(it, disabled) }

// NewSavedMemory builds a user-saved memory item: pinned, full
// importance, tagged SourceUserSaved with the given category. Sibling-
// owned constructor (not part of the contract leaf).
func NewSavedMemory(content string, cat Category) MemoryItem {
	it := MemoryItem{
		Content:    content,
		Importance: 1,
	}
	SetPinned(&it, true)
	SetSource(&it, SourceUserSaved)
	SetCategory(&it, cat)
	return it
}

// NewInferredMemory builds an agent-inferred memory item: unpinned,
// importance clamped to the [0,1] confidence, tagged SourceAgentInferred
// with the given category. Sibling-owned constructor.
func NewInferredMemory(content string, cat Category, confidence float64) MemoryItem {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	it := MemoryItem{
		Content:    content,
		Importance: confidence,
	}
	SetPinned(&it, false)
	SetSource(&it, SourceAgentInferred)
	SetCategory(&it, cat)
	return it
}
