package main

import (
	"sync"
)

// PolicyRule represents a single policy rule in the model.
type PolicyRule []string

// CacheKey represents a cache entry key for an enforcement decision.
type CacheKey struct {
	Params string // JSON serialization of enforcement parameters
}

// CacheEntry represents a cached enforcement decision.
type CacheEntry struct {
	Result bool
	Reason []string
}

// Enforcer is the base enforcer interface that our CachedEnforcer wraps.
// This interface defines the core authorization checking and policy management.
type Enforcer interface {
	// Enforce returns whether a request defined by the given string parameters is allowed.
	Enforce(params ...interface{}) (bool, error)

	// AddPolicy adds a single policy rule to the current model.
	AddPolicy(params ...interface{}) (bool, error)

	// AddPolicies adds multiple policy rules to the current model.
	AddPolicies(rules [][]string) (bool, error)

	// UpdatePolicy updates an existing policy rule in the current model.
	UpdatePolicy(oldRule, newRule []string) (bool, error)

	// RemovePolicy removes a single policy rule from the current model.
	RemovePolicy(params ...interface{}) (bool, error)

	// RemovePolicies removes multiple policy rules from the current model.
	RemovePolicies(rules [][]string) (bool, error)

	// RemoveFilteredPolicy removes policy rules that match the filter from the model.
	RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error)

	// LoadPolicy reloads the policy from the model.
	LoadPolicy() error

	// SetWatcher sets a watcher for the enforcer.
	SetWatcher(watcher Watcher) error

	// SetWatcherEx sets an extended watcher for the enforcer.
	SetWatcherEx(watcher WatcherEx) error
}

// Watcher is an interface for policy change notifications.
type Watcher interface {
	// Update is called when a policy change occurs.
	Update() error
}

// WatcherEx is an extended watcher interface for fine-grained policy change notifications.
type WatcherEx interface {
	Watcher
	// UpdateForUpdatePolicy is called when a policy update occurs.
	UpdateForUpdatePolicy(oldRule, newRule []string) error
	// UpdateForAddPolicy is called when policies are added.
	UpdateForAddPolicy(rule []string) error
	// UpdateForRemovePolicy is called when policies are removed.
	UpdateForRemovePolicy(rule []string) error
	// UpdateForRemoveFilteredPolicy is called when filtered policies are removed.
	UpdateForRemoveFilteredPolicy(fieldIndex int, fieldValues ...string) error
	// UpdateForUpdatePolicies is called when multiple policies are updated.
	UpdateForUpdatePolicies(oldRules, newRules [][]string) error
	// UpdateForRemovePolicies is called when multiple policies are removed.
	UpdateForRemovePolicies(rules [][]string) error
}

// CachedEnforcer wraps an Enforcer and adds caching for Enforce() results.
// It automatically invalidates the cache when policies are modified.
type CachedEnforcer struct {
	// mu protects cache and inner enforcer state
	mu sync.RWMutex

	// inner is the underlying enforcer
	inner Enforcer

	// cache stores the results of Enforce() calls
	cache map[string]CacheEntry

	// watcherCallback is called when the watcher detects policy changes
	watcherCallback Watcher
}

// NewCachedEnforcer creates a new CachedEnforcer wrapping the given enforcer.
func NewCachedEnforcer(inner Enforcer) *CachedEnforcer {
	return &CachedEnforcer{
		inner: inner,
		cache: make(map[string]CacheEntry),
	}
}

// InvalidateCache clears all cached enforcement decisions.
// This is called whenever policies are modified.
func (ce *CachedEnforcer) InvalidateCache() {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.cache = make(map[string]CacheEntry)
}

// InvalidateCacheForKey clears a specific cache entry.
func (ce *CachedEnforcer) InvalidateCacheForKey(key string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	delete(ce.cache, key)
}

// getFromCache retrieves a cached entry if it exists.
func (ce *CachedEnforcer) getFromCache(key string) (CacheEntry, bool) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	entry, exists := ce.cache[key]
	return entry, exists
}

// storeInCache stores an entry in the cache.
func (ce *CachedEnforcer) storeInCache(key string, entry CacheEntry) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.cache[key] = entry
}

// Enforce checks if a request is allowed, using the cache when possible.
// Returns (allowed, error) tuple.
func (ce *CachedEnforcer) Enforce(params ...interface{}) (bool, error) {
	// Generate cache key from parameters
	key := generateCacheKey(params)

	// Check cache first
	if entry, exists := ce.getFromCache(key); exists {
		return entry.Result, nil
	}

	// Cache miss: call underlying enforcer
	result, err := ce.inner.Enforce(params...)
	if err != nil {
		return false, err
	}

	// Store in cache
	ce.storeInCache(key, CacheEntry{Result: result})

	return result, nil
}

// AddPolicy adds a single policy rule and invalidates the cache.
func (ce *CachedEnforcer) AddPolicy(params ...interface{}) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.AddPolicy(params...)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// AddPolicies adds multiple policy rules and invalidates the cache.
func (ce *CachedEnforcer) AddPolicies(rules [][]string) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.AddPolicies(rules)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// UpdatePolicy updates an existing policy rule and invalidates the cache.
func (ce *CachedEnforcer) UpdatePolicy(oldRule, newRule []string) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.UpdatePolicy(oldRule, newRule)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// RemovePolicy removes a single policy rule and invalidates the cache.
func (ce *CachedEnforcer) RemovePolicy(params ...interface{}) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.RemovePolicy(params...)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// RemovePolicies removes multiple policy rules and invalidates the cache.
func (ce *CachedEnforcer) RemovePolicies(rules [][]string) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.RemovePolicies(rules)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// RemoveFilteredPolicy removes policy rules that match the filter and invalidates the cache.
func (ce *CachedEnforcer) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	ce.mu.Lock()
	result, err := ce.inner.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	ce.mu.Unlock()

	if err != nil {
		return false, err
	}

	if result {
		ce.InvalidateCache()
	}

	return result, nil
}

// LoadPolicy reloads the policy from the model and invalidates the cache.
func (ce *CachedEnforcer) LoadPolicy() error {
	ce.mu.Lock()
	err := ce.inner.LoadPolicy()
	ce.mu.Unlock()

	if err != nil {
		return err
	}

	// Always invalidate cache after loading policy
	ce.InvalidateCache()

	return nil
}

// SetWatcher sets a watcher for the enforcer.
// The watcher's Update() call will trigger cache invalidation.
func (ce *CachedEnforcer) SetWatcher(watcher Watcher) error {
	// Wrap the watcher to invalidate cache on updates
	wrappedWatcher := &watcherAdapter{
		watcher:        watcher,
		cacheInvalidate: ce.InvalidateCache,
	}

	ce.mu.Lock()
	err := ce.inner.SetWatcher(wrappedWatcher)
	ce.mu.Unlock()

	if err != nil {
		return err
	}

	ce.mu.Lock()
	ce.watcherCallback = wrappedWatcher
	ce.mu.Unlock()

	return nil
}

// SetWatcherEx sets an extended watcher for the enforcer.
// The watcher's update methods will trigger cache invalidation.
func (ce *CachedEnforcer) SetWatcherEx(watcher WatcherEx) error {
	// Wrap the watcher to invalidate cache on updates
	wrappedWatcher := &watcherExAdapter{
		watcher:         watcher,
		cacheInvalidate: ce.InvalidateCache,
	}

	ce.mu.Lock()
	err := ce.inner.SetWatcherEx(wrappedWatcher)
	ce.mu.Unlock()

	if err != nil {
		return err
	}

	ce.mu.Lock()
	ce.watcherCallback = wrappedWatcher
	ce.mu.Unlock()

	return nil
}

// watcherAdapter wraps a Watcher to invalidate cache on updates.
type watcherAdapter struct {
	watcher         Watcher
	cacheInvalidate func()
}

// Update is called when a policy change is detected.
func (wa *watcherAdapter) Update() error {
	// Invalidate cache when watcher detects changes
	wa.cacheInvalidate()

	// Also call the original watcher's Update
	return wa.watcher.Update()
}

// watcherExAdapter wraps a WatcherEx to invalidate cache on updates.
type watcherExAdapter struct {
	watcher         WatcherEx
	cacheInvalidate func()
}

// Update is called when a policy change is detected.
func (wea *watcherExAdapter) Update() error {
	wea.cacheInvalidate()
	return wea.watcher.Update()
}

// UpdateForUpdatePolicy invalidates cache on policy updates.
func (wea *watcherExAdapter) UpdateForUpdatePolicy(oldRule, newRule []string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForUpdatePolicy(oldRule, newRule)
}

// UpdateForAddPolicy invalidates cache when policies are added.
func (wea *watcherExAdapter) UpdateForAddPolicy(rule []string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForAddPolicy(rule)
}

// UpdateForRemovePolicy invalidates cache when policies are removed.
func (wea *watcherExAdapter) UpdateForRemovePolicy(rule []string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForRemovePolicy(rule)
}

// UpdateForRemoveFilteredPolicy invalidates cache when filtered policies are removed.
func (wea *watcherExAdapter) UpdateForRemoveFilteredPolicy(fieldIndex int, fieldValues ...string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForRemoveFilteredPolicy(fieldIndex, fieldValues...)
}

// UpdateForUpdatePolicies invalidates cache when multiple policies are updated.
func (wea *watcherExAdapter) UpdateForUpdatePolicies(oldRules, newRules [][]string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForUpdatePolicies(oldRules, newRules)
}

// UpdateForRemovePolicies invalidates cache when multiple policies are removed.
func (wea *watcherExAdapter) UpdateForRemovePolicies(rules [][]string) error {
	wea.cacheInvalidate()
	return wea.watcher.UpdateForRemovePolicies(rules)
}
