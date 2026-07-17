/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package utils

import "strconv"

// Small helpers for populating discovery-target attribute maps. Each helper
// wraps the "guard-then-assign" pattern that dominates every extXxx/*_discovery.go
// toTarget function so those functions stay below Sonar's cognitive-complexity
// threshold (go:S3776).

// SetStr assigns attrs[key] = []string{value} only when value is non-empty.
func SetStr(attrs map[string][]string, key, value string) {
	if value != "" {
		attrs[key] = []string{value}
	}
}

// SetBool always assigns attrs[key] = []string{strconv.FormatBool(value)}.
// Booleans have no natural "missing" value in the protos we surface — false is
// a real signal, so this always writes.
func SetBool(attrs map[string][]string, key string, value bool) {
	attrs[key] = []string{strconv.FormatBool(value)}
}

// SetInt64IfPositive assigns the decimal form of value only when value > 0.
// Used for fields the protos default to 0 to signal "unset" (e.g. DataDiskSizeGb,
// MaintenanceWindow.Day).
func SetInt64IfPositive(attrs map[string][]string, key string, value int64) {
	if value > 0 {
		attrs[key] = []string{strconv.FormatInt(value, 10)}
	}
}
