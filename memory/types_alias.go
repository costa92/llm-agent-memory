package memory

import (
	contractmem "github.com/costa92/llm-agent-contract/memory"
)

// This file collapses the sibling-owned public surface onto the leaf
// contract github.com/costa92/llm-agent-contract/memory. Every name
// below is a thin alias / re-export of a contract symbol so the rest
// of this package keeps compiling against bare names while the single
// source of truth for the data + interface contract lives in the
// contract module.

// --- data + interface type aliases ---------------------------------------

type Kind = contractmem.Kind
type MemoryItem = contractmem.MemoryItem
type SearchResult = contractmem.SearchResult
type Stats = contractmem.Stats
type Scope = contractmem.Scope
type Source = contractmem.Source
type Category = contractmem.Category
type ListFilter = contractmem.ListFilter
type ListPage = contractmem.ListPage
type ForgetStrategy = contractmem.ForgetStrategy
type ConsolidateOptions = contractmem.ConsolidateOptions
type ForgetOptions = contractmem.ForgetOptions
type Snapshot = contractmem.Snapshot
type SnapshotItem = contractmem.SnapshotItem
type ImportMode = contractmem.ImportMode
type ImportReport = contractmem.ImportReport
type Embedder = contractmem.Embedder

type Memory = contractmem.Memory
type Lister = contractmem.Lister
type Exporter = contractmem.Exporter
type Importer = contractmem.Importer
type SnapshotStore = contractmem.SnapshotStore

// --- const re-exports -----------------------------------------------------

const (
	KindWorking  = contractmem.KindWorking
	KindEpisodic = contractmem.KindEpisodic
	KindSemantic = contractmem.KindSemantic

	CategoryUser      = contractmem.CategoryUser
	CategoryFeedback  = contractmem.CategoryFeedback
	CategoryProject   = contractmem.CategoryProject
	CategoryReference = contractmem.CategoryReference

	SnapshotVersion = contractmem.SnapshotVersion

	ImportReplace = contractmem.ImportReplace
	ImportMerge   = contractmem.ImportMerge
	ImportUpsert  = contractmem.ImportUpsert

	ForgetByImportance = contractmem.ForgetByImportance
	ForgetByAge        = contractmem.ForgetByAge
	ForgetByCapacity   = contractmem.ForgetByCapacity
)

// --- sentinel re-exports --------------------------------------------------

var (
	ErrKindDisabled               = contractmem.ErrKindDisabled
	ErrSnapshotVersionMismatch    = contractmem.ErrSnapshotVersionMismatch
	ErrSnapshotKindMismatch       = contractmem.ErrSnapshotKindMismatch
	ErrSnapshotStoreNotConfigured = contractmem.ErrSnapshotStoreNotConfigured
)
