package main

import (
	"encoding/json"
	"fmt"
)

// generateCacheKey generates a cache key from enforcement parameters.
// It serializes the parameters to JSON for a consistent, unique key.
func generateCacheKey(params []interface{}) string {
	// For simple cache key generation, we convert params to JSON
	// This allows caching decisions based on the exact parameters
	key, err := json.Marshal(params)
	if err != nil {
		// Fallback to string representation if JSON fails
		return fmt.Sprintf("%v", params)
	}
	return string(key)
}
