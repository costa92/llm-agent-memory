package memory

import coremem "github.com/costa92/llm-agent/memory"

const (
	localMetaKeySource   = "_source"
	localMetaKeyCategory = "_category"
	localMetaKeyPinned   = "_pinned"
	localMetaKeyDisabled = "_disabled"
)

const (
	SourceUserSaved     = coremem.SourceUserSaved
	SourceAgentInferred = coremem.SourceAgentInferred
	SourceSystem        = coremem.SourceSystem
	SourceUnknown       = coremem.SourceUnknown
)

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

func GetSource(it MemoryItem) Source {
	if it.Metadata == nil {
		return SourceUnknown
	}
	raw, ok := it.Metadata[localMetaKeySource]
	if !ok {
		return SourceUnknown
	}
	s, ok := raw.(string)
	if !ok {
		return SourceUnknown
	}
	return Source(s)
}

func SetSource(it *MemoryItem, src Source) {
	ensureMetadata(it)
	if src == SourceUnknown {
		delete(it.Metadata, localMetaKeySource)
		return
	}
	it.Metadata[localMetaKeySource] = string(src)
}

func GetCategory(it MemoryItem) Category {
	if it.Metadata == nil {
		return ""
	}
	raw, ok := it.Metadata[localMetaKeyCategory]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return Category(s)
}

func SetCategory(it *MemoryItem, cat Category) {
	ensureMetadata(it)
	if cat == "" {
		delete(it.Metadata, localMetaKeyCategory)
		return
	}
	it.Metadata[localMetaKeyCategory] = string(cat)
}

func IsPinned(it MemoryItem) bool {
	if it.Metadata == nil {
		return false
	}
	raw, ok := it.Metadata[localMetaKeyPinned]
	if !ok {
		return false
	}
	pinned, ok := raw.(bool)
	return ok && pinned
}

func SetPinned(it *MemoryItem, pinned bool) {
	ensureMetadata(it)
	if !pinned {
		delete(it.Metadata, localMetaKeyPinned)
		return
	}
	it.Metadata[localMetaKeyPinned] = true
}

func IsDisabled(it MemoryItem) bool {
	if it.Metadata == nil {
		return false
	}
	raw, ok := it.Metadata[localMetaKeyDisabled]
	if !ok {
		return false
	}
	disabled, ok := raw.(bool)
	return ok && disabled
}

func SetDisabled(it *MemoryItem, disabled bool) {
	ensureMetadata(it)
	if !disabled {
		delete(it.Metadata, localMetaKeyDisabled)
		return
	}
	it.Metadata[localMetaKeyDisabled] = true
}

func ensureMetadata(it *MemoryItem) {
	if it.Metadata == nil {
		it.Metadata = make(map[string]any, 4)
	}
}
