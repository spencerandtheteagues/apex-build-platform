package storage

import (
	"context"
	"io"
	"time"
)

// Provider is the storage backend interface. R2 is the default; local disk is the fallback.
type Provider interface {
	// Put uploads content. key is the object path (e.g. "projects/123/uuid.png")
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get returns a reader for the object. Caller must close it.
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	// Delete removes an object.
	Delete(ctx context.Context, key string) error
	// URL returns a presigned URL valid for ttl duration.
	URL(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Exists returns true if the object exists.
	Exists(ctx context.Context, key string) (bool, error)
}