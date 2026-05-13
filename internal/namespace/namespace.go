// Package namespace provides the 4-part namespace key for Mem0 memory
// isolation (Race Layer 3). Each key scopes reads and writes so that
// concurrent agents with different run_ids cannot see each other's state.
//
// Format: {app_id}/{user_id}/{namespace}/{run_id}
//
// The namespace concept maps directly to Mem0's existing app_id/user_id
// schema, extended with an explicit namespace segment and a run_id for
// per-invocation isolation.
package namespace

import (
	"fmt"
	"strings"
)

// Key represents a 4-part namespace for Mem0 memory scoping.
type Key struct {
	AppID     string
	UserID    string
	Namespace string
	RunID     string
}

// New creates a typed namespace key. All parts are required; empty parts
// produce a deterministic but semantically meaningless key segment (the
// caller should validate before persisting).
func New(appID, userID, ns, runID string) Key {
	return Key{
		AppID:     appID,
		UserID:    userID,
		Namespace: ns,
		RunID:     runID,
	}
}

// String encodes the key as the canonical 4-part string.
func (k Key) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", k.AppID, k.UserID, k.Namespace, k.RunID)
}

// Parse decodes a 4-part string into a Key. Returns an error if the
// string does not have exactly 4 slash-separated segments.
func Parse(s string) (Key, error) {
	parts := strings.SplitN(s, "/", 4)
	if len(parts) != 4 {
		return Key{}, fmt.Errorf("namespace: expected 4 parts, got %d in %q", len(parts), s)
	}
	return Key{
		AppID:     parts[0],
		UserID:    parts[1],
		Namespace: parts[2],
		RunID:     parts[3],
	}, nil
}

// Matches returns true if the key matches the given filter. Empty filter
// fields match any value (wildcard).
func (k Key) Matches(filter Key) bool {
	if filter.AppID != "" && filter.AppID != k.AppID {
		return false
	}
	if filter.UserID != "" && filter.UserID != k.UserID {
		return false
	}
	if filter.Namespace != "" && filter.Namespace != k.Namespace {
		return false
	}
	if filter.RunID != "" && filter.RunID != k.RunID {
		return false
	}
	return true
}

// MetadataMap returns the key as a map suitable for Mem0 metadata fields.
func (k Key) MetadataMap() map[string]string {
	return map[string]string{
		"app_id":        k.AppID,
		"user_id":       k.UserID,
		"namespace":     k.Namespace,
		"run_id":        k.RunID,
		"namespace_key": k.String(),
	}
}

// Predefined namespace values for built-in surfaces.
const (
	NSCallbacks    = "callbacks"
	NSCheckpoint   = "checkpoint"
	NSEvoloop      = "evoloop"
	NSCoordination = "coordination"
)
