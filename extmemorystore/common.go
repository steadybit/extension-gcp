/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmemorystore

const (
	TargetIDRedisInstance = "com.steadybit.extension_gcp.memorystore.redis-instance"
	RedisFailoverActionId = "com.steadybit.extension_gcp.memorystore.redis-instance.failover"
	targetIcon            = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgM2M0Ljk3IDAgOSAxLjM0IDkgM3YxMmMwIDEuNjYtNC4wMyAzLTkgM3MtOS0xLjM0LTktM1Y2YzAtMS42NiA0LjAzLTMgOS0zem0wIDJjLTMuOSAwLTcgLjg5LTcgMnMzLjEgMiA3IDIgNy0uODkgNy0yLTMuMS0yLTctMnoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPjwvc3ZnPg=="

	// Attribute names extracted per Sonar go:S1192.
	attrTier         = "gcp.memorystore.tier"
	attrRedisVersion = "gcp.memorystore.redis-version"
	attrRegion       = "gcp.memorystore.region"
	attrProjectID    = "gcp.project.id"
)
