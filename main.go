package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Casbin CachedEnforcer - Authorization Caching with Policy Invalidation")
	fmt.Println()

	// Create a mock enforcer for demonstration
	mockEnforcer := NewMockEnforcer()
	mockEnforcer.policies["[\"alice\",\"read\",\"data1\"]"] = true
	mockEnforcer.policies["[\"bob\",\"write\",\"data2\"]"] = true

	// Create a cached enforcer wrapping the mock
	cachedEnforcer := NewCachedEnforcer(mockEnforcer)

	// Example 1: Cache hit
	fmt.Println("Example 1: Cache Hit")
	fmt.Println("-------------------")
	result1, err := cachedEnforcer.Enforce("alice", "read", "data1")
	if err != nil {
		log.Fatalf("Enforce failed: %v", err)
	}
	fmt.Printf("First call - Enforce(alice, read, data1): %v (Underlying calls: %d)\n", result1, mockEnforcer.GetEnforceCallCount())

	result2, err := cachedEnforcer.Enforce("alice", "read", "data1")
	if err != nil {
		log.Fatalf("Enforce failed: %v", err)
	}
	fmt.Printf("Second call - Enforce(alice, read, data1): %v (Underlying calls: %d - cached!)\n", result2, mockEnforcer.GetEnforceCallCount())
	fmt.Println()

	// Example 2: Cache invalidation on policy modification
	fmt.Println("Example 2: Cache Invalidation on Policy Change")
	fmt.Println("----------------------------------------------")
	fmt.Println("Adding new policy: (charlie, read, data3)")
	added, err := cachedEnforcer.AddPolicy("charlie", "read", "data3")
	if err != nil {
		log.Fatalf("AddPolicy failed: %v", err)
	}
	fmt.Printf("Policy added: %v, Cache invalidated\n", added)
	fmt.Println()

	// Example 3: Multiple policy modifications
	fmt.Println("Example 3: Multiple Policy Modifications")
	fmt.Println("--------------------------------------")
	fmt.Println("Removing policy: (bob, write, data2)")
	removed, err := cachedEnforcer.RemovePolicy("bob", "write", "data2")
	if err != nil {
		log.Fatalf("RemovePolicy failed: %v", err)
	}
	fmt.Printf("Policy removed: %v, Cache invalidated\n", removed)
	fmt.Println()

	// Example 4: LoadPolicy invalidation
	fmt.Println("Example 4: LoadPolicy Invalidation")
	fmt.Println("---------------------------------")
	fmt.Println("Loading policies from adapter")
	err = cachedEnforcer.LoadPolicy()
	if err != nil {
		log.Fatalf("LoadPolicy failed: %v", err)
	}
	fmt.Printf("Policies loaded, cache invalidated\n")
	fmt.Println()

	fmt.Println("CachedEnforcer features:")
	fmt.Println("✓ Caches Enforce() results for performance")
	fmt.Println("✓ Automatically invalidates cache on policy changes")
	fmt.Println("✓ Thread-safe with sync.RWMutex")
	fmt.Println("✓ Supports policy watchers for distributed invalidation")
	fmt.Println("✓ No stale authorization decisions")
}
