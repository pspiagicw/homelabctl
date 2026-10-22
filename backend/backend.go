// Package backend holds the interfaces the daemon's components use to call
// into the orchestrator. Each consumer depends only on the small interface it
// needs; the orchestrator implements all of them.
package backend

import "context"

// API is what the REST server needs. Results are returned as-is and encoded
// to JSON; errors are mapped to HTTP status codes via apierr.
type API interface {
	Status(ctx context.Context) (any, error)
	ListNodes(ctx context.Context) (any, error)
	GetNode(ctx context.Context, name string) (any, error)
	Boot(ctx context.Context, name string) (any, error)
	Shutdown(ctx context.Context, name string) (any, error)
	ListServices(ctx context.Context) (any, error)
	RefreshNodes(ctx context.Context) (any, error)
}

// Power is what the sentinel needs to react to mains power changes.
type Power interface {
	// ShutdownAll shuts down every managed node (outage confirmed).
	ShutdownAll(ctx context.Context)
	// RestoreAll runs once mains power is back and stable.
	RestoreAll(ctx context.Context)
}
