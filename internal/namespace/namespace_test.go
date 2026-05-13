package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_AllFields(t *testing.T) {
	k := New("cursor-global-kb", "nfsarch33", "callbacks", "run-42")
	assert.Equal(t, "cursor-global-kb", k.AppID)
	assert.Equal(t, "nfsarch33", k.UserID)
	assert.Equal(t, "callbacks", k.Namespace)
	assert.Equal(t, "run-42", k.RunID)
}

func TestKey_String(t *testing.T) {
	k := New("app", "user", "ns", "run")
	assert.Equal(t, "app/user/ns/run", k.String())
}

func TestKey_String_Deterministic(t *testing.T) {
	k1 := New("a", "b", "c", "d")
	k2 := New("a", "b", "c", "d")
	assert.Equal(t, k1.String(), k2.String())
}

func TestParse_Valid(t *testing.T) {
	k, err := Parse("app/user/ns/run")
	require.NoError(t, err)
	assert.Equal(t, "app", k.AppID)
	assert.Equal(t, "user", k.UserID)
	assert.Equal(t, "ns", k.Namespace)
	assert.Equal(t, "run", k.RunID)
}

func TestParse_Roundtrip(t *testing.T) {
	original := New("cursor-global-kb", "nfsarch33", "checkpoint", "run-99")
	parsed, err := Parse(original.String())
	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestParse_TooFewParts(t *testing.T) {
	_, err := Parse("app/user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 4 parts")
}

func TestParse_TooManyPartsCollapsesIntoRunID(t *testing.T) {
	k, err := Parse("app/user/ns/run/extra/parts")
	require.NoError(t, err)
	assert.Equal(t, "run/extra/parts", k.RunID)
}

func TestMatches_ExactMatch(t *testing.T) {
	k := New("app", "user", "ns", "run")
	filter := New("app", "user", "ns", "run")
	assert.True(t, k.Matches(filter))
}

func TestMatches_WildcardRunID(t *testing.T) {
	k := New("app", "user", "ns", "run-42")
	filter := New("app", "user", "ns", "")
	assert.True(t, k.Matches(filter), "empty RunID should match any")
}

func TestMatches_WildcardAll(t *testing.T) {
	k := New("app", "user", "ns", "run")
	assert.True(t, k.Matches(Key{}), "empty filter should match everything")
}

func TestMatches_Mismatch(t *testing.T) {
	k := New("app", "user", "ns", "run")
	filter := New("other-app", "", "", "")
	assert.False(t, k.Matches(filter))
}

func TestMetadataMap(t *testing.T) {
	k := New("app1", "user1", "evoloop", "cycle-7")
	m := k.MetadataMap()

	assert.Equal(t, "app1", m["app_id"])
	assert.Equal(t, "user1", m["user_id"])
	assert.Equal(t, "evoloop", m["namespace"])
	assert.Equal(t, "cycle-7", m["run_id"])
	assert.Equal(t, "app1/user1/evoloop/cycle-7", m["namespace_key"])
}

func TestPredefinedNamespaces(t *testing.T) {
	assert.Equal(t, "callbacks", NSCallbacks)
	assert.Equal(t, "checkpoint", NSCheckpoint)
	assert.Equal(t, "evoloop", NSEvoloop)
	assert.Equal(t, "coordination", NSCoordination)
}

func TestConcurrentRunsIsolated(t *testing.T) {
	run1 := New("cursor-global-kb", "nfsarch33", "checkpoint", "run-1")
	run2 := New("cursor-global-kb", "nfsarch33", "checkpoint", "run-2")

	filter1 := New("cursor-global-kb", "nfsarch33", "checkpoint", "run-1")
	filter2 := New("cursor-global-kb", "nfsarch33", "checkpoint", "run-2")

	assert.True(t, run1.Matches(filter1))
	assert.False(t, run1.Matches(filter2), "run-1 must NOT match run-2 filter")
	assert.True(t, run2.Matches(filter2))
	assert.False(t, run2.Matches(filter1), "run-2 must NOT match run-1 filter")
}
