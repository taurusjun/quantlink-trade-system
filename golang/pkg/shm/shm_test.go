// Package shm provides shared memory utilities tests
package shm

import (
	"testing"
)

// TestTVarCreation tests TVar struct creation
// Note: Actual shared memory operations require valid keys and may not work in all environments
func TestTVarCreation(t *testing.T) {
	// Test with invalid key (should return error)
	_, err := NewTVar(0)
	if err == nil {
		t.Error("Expected error for invalid key 0")
	}

	_, err = NewTVar(-1)
	if err == nil {
		t.Error("Expected error for invalid key -1")
	}
}

// TestTCacheCreation tests TCache struct creation
// Note: Actual shared memory operations require valid keys and may not work in all environments
func TestTCacheCreation(t *testing.T) {
	// Test with invalid key (should return error)
	_, err := NewTCache(0, 100)
	if err == nil {
		t.Error("Expected error for invalid key 0")
	}

	_, err = NewTCache(-1, 100)
	if err == nil {
		t.Error("Expected error for invalid key -1")
	}
}

// TestTCacheNodeSize tests that TCacheNode has expected size
func TestTCacheNodeSize(t *testing.T) {
	// Key should be 64 bytes, Value should be 8 bytes (float64)
	// Total: 72 bytes
	node := TCacheNode{}
	if len(node.Key) != 64 {
		t.Errorf("Expected Key size 64, got %d", len(node.Key))
	}
}

// TestCstrLen tests the cstrLen helper function
func TestCstrLen(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected int
	}{
		{[]byte("hello\x00world"), 5},
		{[]byte("\x00"), 0},
		{[]byte("test"), 4},
		{[]byte{}, 0},
	}

	for _, tc := range testCases {
		result := cstrLen(tc.input)
		if result != tc.expected {
			t.Errorf("cstrLen(%v) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}
