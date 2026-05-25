package memory

import (
	"errors"
	"testing"
)

func TestNewScopedLifecycleManager_NilInner_ReturnsSentinel(t *testing.T) {
	if _, err := NewScopedLifecycleManager(nil); !errors.Is(err, ErrScopedManagerRequired) {
		t.Errorf("NewScopedLifecycleManager(nil) err = %v, want ErrScopedManagerRequired", err)
	}
}

func TestNewConsolidator_NilInner_ReturnsSentinel(t *testing.T) {
	if _, err := NewConsolidator(nil); !errors.Is(err, ErrManagerRequired) {
		t.Errorf("NewConsolidator(nil) err = %v, want ErrManagerRequired", err)
	}
}

func TestNewUnifiedSearcher_NilInner_ReturnsSentinel(t *testing.T) {
	if _, err := NewUnifiedSearcher(nil); !errors.Is(err, ErrUnifiedManagerRequired) {
		t.Errorf("NewUnifiedSearcher(nil) err = %v, want ErrUnifiedManagerRequired", err)
	}
}
