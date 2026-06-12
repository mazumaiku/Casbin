package main

// This file contains examples of using CachedEnforcer in various scenarios.
// These examples demonstrate best practices and integration patterns.

/*
EXAMPLE 1: Basic Usage

```go
// Assuming you have an existing Enforcer implementation
baseEnforcer := createYourEnforcer()

// Wrap it with CachedEnforcer
cached := NewCachedEnforcer(baseEnforcer)

// Use it exactly like the base enforcer
allowed, err := cached.Enforce("alice", "read", "data1")
if err != nil {
    log.Fatal(err)
}

if allowed {
    // Grant access
} else {
    // Deny access
}

// Cache automatically invalidates when policies change
cached.AddPolicy("bob", "write", "data2")  // Cache cleared
```

EXAMPLE 2: With Policy Watcher

```go
// Create cached enforcer
cached := NewCachedEnforcer(baseEnforcer)

// Set up a policy watcher for distributed invalidation
watcher := &MyPolicyWatcher{
    redisClient: redisClient,
}

err := cached.SetWatcher(watcher)
if err != nil {
    log.Fatal(err)
}

// Now when another instance updates policies, the watcher
// will notify this instance to invalidate its cache
```

EXAMPLE 3: Concurrent Usage Pattern

```go
// CachedEnforcer is safe for concurrent use
cached := NewCachedEnforcer(baseEnforcer)

// Many goroutines can call Enforce concurrently
for i := 0; i < 1000; i++ {
    go func(userID string) {
        allowed, _ := cached.Enforce(userID, "read", "data")
        // Use authorization decision
    }("user" + string(rune(i)))
}

// While another goroutine updates policies
go func() {
    time.Sleep(100 * time.Millisecond)
    cached.AddPolicy("newuser", "admin", "")  // Cache cleared safely
}()
```

EXAMPLE 4: Manual Cache Control

```go
cached := NewCachedEnforcer(baseEnforcer)

// Populate cache with enforcement decisions
cached.Enforce("alice", "read", "data1")
cached.Enforce("bob", "write", "data2")

// Clear specific cache entry if needed
key := generateCacheKey([]interface{}{"alice", "read", "data1"})
cached.InvalidateCacheForKey(key)

// Or clear entire cache
cached.InvalidateCache()

// Next Enforce call will hit the underlying enforcer
allowed, _ := cached.Enforce("alice", "read", "data1")
```

EXAMPLE 5: Batch Policy Updates

```go
cached := NewCachedEnforcer(baseEnforcer)

// Add multiple policies at once
policies := [][]string{
    {"alice", "read", "data1"},
    {"bob", "write", "data2"},
    {"charlie", "delete", "data3"},
}

changed, err := cached.AddPolicies(policies)
if err != nil {
    log.Fatal(err)
}

if changed {
    // Cache was automatically invalidated
    fmt.Println("Policies updated, cache cleared")
}
```

EXAMPLE 6: Policy Reloading from Adapter

```go
cached := NewCachedEnforcer(baseEnforcer)

// Load policies from an external source (database, file, etc.)
err := cached.LoadPolicy()
if err != nil {
    log.Fatal(err)
}

// Cache is automatically cleared, next Enforce will evaluate
// against the newly loaded policies
allowed, _ := cached.Enforce("alice", "read", "data1")
```

EXAMPLE 7: Monitoring Cache Effectiveness

```go
// Create a mock enforcer to count underlying calls
mockEnforcer := NewMockEnforcer()
cached := NewCachedEnforcer(mockEnforcer)

// Simulate enforcement operations
for i := 0; i < 1000; i++ {
    cached.Enforce("alice", "read", "data1")
}

underlyingCalls := mockEnforcer.GetEnforceCallCount()
cacheHitRate := (1000 - underlyingCalls) / 1000.0

fmt.Printf("Cache hit rate: %.2f%%\n", cacheHitRate*100)
// Output: Cache hit rate: 99.90% (only 1 underlying call for 1000 Enforce calls)
```

PERFORMANCE CHARACTERISTICS:

Cache Hit: O(1) - single map lookup with RLock
Cache Miss: O(1) or O(n) - depends on underlying enforcer complexity

Write Operations:
- AddPolicy: Underlying operation + O(1) cache clear
- RemovePolicy: Underlying operation + O(1) cache clear
- UpdatePolicy: Underlying operation + O(1) cache clear
- LoadPolicy: Underlying load operation + O(1) cache clear

Thread Safety: All operations are protected by sync.RWMutex with no deadlock risk

MIGRATION GUIDE:

From unencached enforcer to CachedEnforcer:

// Before:
enforcer := createEnforcer()
allowed, _ := enforcer.Enforce("alice", "read", "data")

// After:
enforcer := createEnforcer()
cached := NewCachedEnforcer(enforcer)
allowed, _ := cached.Enforce("alice", "read", "data")

No other code changes needed! CachedEnforcer implements the same interface.
*/
