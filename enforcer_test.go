package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// MockEnforcer is a mock implementation of Enforcer for testing.
type MockEnforcer struct {
	mu                    sync.RWMutex
	policies              map[string]bool // key: policy rule, value: whether it allows
	enforceCallCount      int32           // atomic counter for calls
	addPolicyCallCount    int32
	removePolicyCallCount int32
	loadPolicyCallCount   int32
	shouldFail            bool
}

func NewMockEnforcer() *MockEnforcer {
	return &MockEnforcer{
		policies: make(map[string]bool),
	}
}

func (me *MockEnforcer) Enforce(params ...interface{}) (bool, error) {
	atomic.AddInt32(&me.enforceCallCount, 1)

	if me.shouldFail {
		return false, errors.New("enforce failed")
	}

	me.mu.RLock()
	defer me.mu.RUnlock()

	// Simple mock: create a key from params
	key := generateCacheKey(params)
	result, exists := me.policies[key]
	if !exists {
		return false, nil
	}
	return result, nil
}

func (me *MockEnforcer) AddPolicy(params ...interface{}) (bool, error) {
	atomic.AddInt32(&me.addPolicyCallCount, 1)

	me.mu.Lock()
	defer me.mu.Unlock()

	key := generateCacheKey(params)
	if _, exists := me.policies[key]; exists {
		return false, nil // Already exists
	}
	me.policies[key] = true
	return true, nil
}

func (me *MockEnforcer) AddPolicies(rules [][]string) (bool, error) {
	me.mu.Lock()
	defer me.mu.Unlock()

	changed := false
	for _, rule := range rules {
		key := generateCacheKey(convertToInterface(rule))
		if _, exists := me.policies[key]; !exists {
			me.policies[key] = true
			changed = true
		}
	}
	return changed, nil
}

func (me *MockEnforcer) UpdatePolicy(oldRule, newRule []string) (bool, error) {
	me.mu.Lock()
	defer me.mu.Unlock()

	oldKey := generateCacheKey(convertToInterface(oldRule))
	newKey := generateCacheKey(convertToInterface(newRule))

	if _, exists := me.policies[oldKey]; !exists {
		return false, nil // Old rule doesn't exist
	}

	delete(me.policies, oldKey)
	me.policies[newKey] = true
	return true, nil
}

func (me *MockEnforcer) RemovePolicy(params ...interface{}) (bool, error) {
	atomic.AddInt32(&me.removePolicyCallCount, 1)

	me.mu.Lock()
	defer me.mu.Unlock()

	key := generateCacheKey(params)
	if _, exists := me.policies[key]; !exists {
		return false, nil // Doesn't exist
	}
	delete(me.policies, key)
	return true, nil
}

func (me *MockEnforcer) RemovePolicies(rules [][]string) (bool, error) {
	me.mu.Lock()
	defer me.mu.Unlock()

	changed := false
	for _, rule := range rules {
		key := generateCacheKey(convertToInterface(rule))
		if _, exists := me.policies[key]; exists {
			delete(me.policies, key)
			changed = true
		}
	}
	return changed, nil
}

func (me *MockEnforcer) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	me.mu.Lock()
	defer me.mu.Unlock()

	changed := false
	// Simple filter: remove all policies containing the field values
	for key := range me.policies {
		for _, val := range fieldValues {
			if len(key) > 0 && val != "" {
				// Very simple matching for test purposes
				changed = true
				delete(me.policies, key)
				break
			}
		}
	}
	return changed, nil
}

func (me *MockEnforcer) LoadPolicy() error {
	atomic.AddInt32(&me.loadPolicyCallCount, 1)

	me.mu.Lock()
	defer me.mu.Unlock()

	// Simulate loading: clear and reload policies
	me.policies = make(map[string]bool)
	return nil
}

func (me *MockEnforcer) SetWatcher(watcher Watcher) error {
	return nil
}

func (me *MockEnforcer) SetWatcherEx(watcher WatcherEx) error {
	return nil
}

func (me *MockEnforcer) GetEnforceCallCount() int32 {
	return atomic.LoadInt32(&me.enforceCallCount)
}

func (me *MockEnforcer) GetAddPolicyCallCount() int32 {
	return atomic.LoadInt32(&me.addPolicyCallCount)
}

func (me *MockEnforcer) GetRemovePolicyCallCount() int32 {
	return atomic.LoadInt32(&me.removePolicyCallCount)
}

func convertToInterface(arr []string) []interface{} {
	result := make([]interface{}, len(arr))
	for i, v := range arr {
		result[i] = v
	}
	return result
}

// Tests

func TestCachedEnforcer_CacheHit(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// First call should hit the underlying enforcer
	result1, err := cached.Enforce("user1", "read", "doc1")
	if err != nil {
		t.Fatalf("First Enforce failed: %v", err)
	}
	if !result1 {
		t.Fatal("Expected true from first Enforce call")
	}
	callCount1 := mock.GetEnforceCallCount()
	if callCount1 != 1 {
		t.Fatalf("Expected 1 enforce call, got %d", callCount1)
	}

	// Second call should hit the cache
	result2, err := cached.Enforce("user1", "read", "doc1")
	if err != nil {
		t.Fatalf("Second Enforce failed: %v", err)
	}
	if !result2 {
		t.Fatal("Expected true from second Enforce call")
	}
	callCount2 := mock.GetEnforceCallCount()
	if callCount2 != 1 {
		t.Fatalf("Expected 1 enforce call (cached), got %d", callCount2)
	}
}

func TestCachedEnforcer_CacheInvalidateOnAddPolicy(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 1 {
		t.Fatal("Cache not populated")
	}

	// Add a policy
	added, err := cached.AddPolicy("user2", "write", "doc2")
	if err != nil || !added {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true // Re-add for next Enforce
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 2 {
		t.Fatalf("Cache not invalidated after AddPolicy. Expected 2 calls, got %d", mock.GetEnforceCallCount())
	}
}

func TestCachedEnforcer_CacheInvalidateOnRemovePolicy(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 1 {
		t.Fatal("Cache not populated")
	}

	// Remove a policy
	removed, err := cached.RemovePolicy("user1", "read", "doc1")
	if err != nil || !removed {
		t.Fatalf("RemovePolicy failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = false // Update for next Enforce
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 2 {
		t.Fatalf("Cache not invalidated after RemovePolicy. Expected 2 calls, got %d", mock.GetEnforceCallCount())
	}
}

func TestCachedEnforcer_CacheInvalidateOnUpdatePolicy(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 1 {
		t.Fatal("Cache not populated")
	}

	// Update a policy
	updated, err := cached.UpdatePolicy(
		[]string{"user1", "read", "doc1"},
		[]string{"user1", "write", "doc1"},
	)
	if err != nil || !updated {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true // Re-add for next Enforce
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 2 {
		t.Fatalf("Cache not invalidated after UpdatePolicy. Expected 2 calls, got %d", mock.GetEnforceCallCount())
	}
}

func TestCachedEnforcer_CacheInvalidateOnLoadPolicy(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 1 {
		t.Fatal("Cache not populated")
	}

	// Load policy
	err := cached.LoadPolicy()
	if err != nil {
		t.Fatalf("LoadPolicy failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true // Re-add for next Enforce
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() != 2 {
		t.Fatalf("Cache not invalidated after LoadPolicy. Expected 2 calls, got %d", mock.GetEnforceCallCount())
	}
}

func TestCachedEnforcer_CacheInvalidateOnAddPolicies(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	initialCallCount := mock.GetEnforceCallCount()

	// Add multiple policies
	added, err := cached.AddPolicies([][]string{
		{"user2", "write", "doc2"},
		{"user3", "delete", "doc3"},
	})
	if err != nil || !added {
		t.Fatalf("AddPolicies failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() == initialCallCount {
		t.Fatalf("Cache not invalidated after AddPolicies")
	}
}

func TestCachedEnforcer_CacheInvalidateOnRemovePolicies(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	initialCallCount := mock.GetEnforceCallCount()

	// Remove multiple policies
	removed, err := cached.RemovePolicies([][]string{
		{"user1", "read", "doc1"},
	})
	if err != nil || !removed {
		t.Fatalf("RemovePolicies failed: %v", err)
	}

	// Cache should be invalidated
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = false
	cached.Enforce("user1", "read", "doc1")
	if mock.GetEnforceCallCount() == initialCallCount {
		t.Fatalf("Cache not invalidated after RemovePolicies")
	}
}

func TestCachedEnforcer_CacheInvalidateOnRemoveFilteredPolicy(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	initialCallCount := mock.GetEnforceCallCount()

	// Remove filtered policies
	removed, err := cached.RemoveFilteredPolicy(0, "user1")
	if err != nil {
		t.Fatalf("RemoveFilteredPolicy failed: %v", err)
	}

	if removed {
		// Cache should be invalidated only if something was actually removed
		mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true
		cached.Enforce("user1", "read", "doc1")
		if mock.GetEnforceCallCount() == initialCallCount {
			t.Fatalf("Cache not invalidated after RemoveFilteredPolicy")
		}
	}
}

func TestCachedEnforcer_ThreadSafety(t *testing.T) {
	mock := NewMockEnforcer()
	for i := 0; i < 100; i++ {
		key := generateCacheKey([]interface{}{i, "read", "doc"})
		mock.policies[key] = true
	}

	cached := NewCachedEnforcer(mock)

	// Concurrent reads and writes
	var wg sync.WaitGroup
	numGoroutines := 20
	operationsPerGoroutine := 50

	// Writer goroutines
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				if j%2 == 0 {
					cached.AddPolicy("user", "action", "resource")
				} else {
					cached.RemovePolicy("user", "action", "resource")
				}
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				cached.Enforce(idx, "read", "doc")
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panic or race condition, test passes
}

func TestCachedEnforcer_DifferentParameters(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true
	mock.policies["[\"user2\",\"write\",\"doc2\"]"] = false

	cached := NewCachedEnforcer(mock)

	// Different parameters should have separate cache entries
	result1, _ := cached.Enforce("user1", "read", "doc1")
	result2, _ := cached.Enforce("user2", "write", "doc2")

	if result1 != true || result2 != false {
		t.Fatal("Different parameters returned wrong results")
	}

	callCount := mock.GetEnforceCallCount()
	if callCount != 2 {
		t.Fatalf("Expected 2 calls for different parameters, got %d", callCount)
	}

	// Enforce same params again - should hit cache
	result1Again, _ := cached.Enforce("user1", "read", "doc1")
	result2Again, _ := cached.Enforce("user2", "write", "doc2")

	if result1Again != true || result2Again != false {
		t.Fatal("Cached results don't match original")
	}

	callCountAfter := mock.GetEnforceCallCount()
	if callCountAfter != 2 {
		t.Fatalf("Expected 2 calls (cached), got %d", callCountAfter)
	}
}

func TestCachedEnforcer_InvalidateCacheForKey(t *testing.T) {
	mock := NewMockEnforcer()
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true

	cached := NewCachedEnforcer(mock)

	// Populate cache
	cached.Enforce("user1", "read", "doc1")
	initialCount := mock.GetEnforceCallCount()

	// Invalidate specific cache entry
	key := generateCacheKey([]interface{}{"user1", "read", "doc1"})
	cached.InvalidateCacheForKey(key)

	// Should call enforcer again
	mock.policies["[\"user1\",\"read\",\"doc1\"]"] = true
	cached.Enforce("user1", "read", "doc1")
	finalCount := mock.GetEnforceCallCount()

	if finalCount != initialCount+1 {
		t.Fatalf("Expected %d calls after invalidation, got %d", initialCount+1, finalCount)
	}
}

func TestCachedEnforcer_ErrorHandling(t *testing.T) {
	mock := NewMockEnforcer()
	mock.shouldFail = true

	cached := NewCachedEnforcer(mock)

	// Should return error from underlying enforcer
	_, err := cached.Enforce("user1", "read", "doc1")
	if err == nil {
		t.Fatal("Expected error from enforcer, got none")
	}

	// Cache should not contain failed result
	callCount1 := mock.GetEnforceCallCount()
	_, err = cached.Enforce("user1", "read", "doc1")
	callCount2 := mock.GetEnforceCallCount()

	if callCount2 == callCount1 {
		t.Fatal("Failed result was cached, should retry on next call")
	}
}
