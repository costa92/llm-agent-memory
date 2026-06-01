package memory

import "context"

type scopeCtxKey struct{}

// WithScope returns a child context carrying the provided scope.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeCtxKey{}, s)
}

// ScopeFrom extracts a scope from context, returning zero-value Scope when absent.
func ScopeFrom(ctx context.Context) Scope {
	v, ok := ctx.Value(scopeCtxKey{}).(Scope)
	if !ok {
		return Scope{}
	}
	return v
}

func stampScope(item *MemoryItem, s Scope) {
	if s.IsZero() {
		return
	}
	if item.Metadata == nil {
		item.Metadata = make(map[string]any, 1)
	}
	item.Metadata[localMetaKeyScope] = map[string]string{
		"user":    s.User,
		"project": s.Project,
		"session": s.Session,
	}
}
