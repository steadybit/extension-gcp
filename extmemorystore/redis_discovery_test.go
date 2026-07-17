/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmemorystore

import (
	"testing"

	"cloud.google.com/go/redis/apiv1/redispb"
	"github.com/stretchr/testify/assert"
)

func TestToRedisTarget_Populated(t *testing.T) {
	inst := &redispb.Instance{
		Name:                  "projects/proj-a/locations/europe-west1/instances/cache-01",
		Tier:                  redispb.Instance_STANDARD_HA,
		RedisVersion:          "REDIS_6_X",
		LocationId:            "europe-west1-a",
		AlternativeLocationId: "europe-west1-b",
		MemorySizeGb:          8,
		State:                 redispb.Instance_READY,
		ConnectMode:           redispb.Instance_PRIVATE_SERVICE_ACCESS,
		AuthEnabled:           true,
		TransitEncryptionMode: redispb.Instance_SERVER_AUTHENTICATION,
		ReadReplicasMode:      redispb.Instance_READ_REPLICAS_ENABLED,
		ReplicaCount:          2,
		AuthorizedNetwork:     "projects/proj-a/global/networks/default",
		PersistenceConfig: &redispb.PersistenceConfig{
			PersistenceMode: redispb.PersistenceConfig_RDB,
		},
		Labels: map[string]string{"team": "core"},
	}

	target := toRedisTarget(inst, "proj-a")

	assert.Equal(t, TargetIDRedisInstance, target.TargetType)
	assert.Equal(t, "cache-01", target.Label)
	assert.Equal(t, inst.Name, target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"cache-01"}, target.Attributes["gcp.memorystore.instance.id"])
	assert.Equal(t, []string{"europe-west1"}, target.Attributes[attrRegion])
	assert.Equal(t, []string{"STANDARD_HA"}, target.Attributes[attrTier])
	assert.Equal(t, []string{"REDIS_6_X"}, target.Attributes[attrRedisVersion])
	assert.Equal(t, []string{"europe-west1-a"}, target.Attributes["gcp.memorystore.location-id"])
	assert.Equal(t, []string{"europe-west1-b"}, target.Attributes["gcp.memorystore.alternative-location-id"])
	assert.Equal(t, []string{"8"}, target.Attributes["gcp.memorystore.memory-size-gb"])
	assert.Equal(t, []string{"READY"}, target.Attributes["gcp.memorystore.state"])
	assert.Equal(t, []string{"PRIVATE_SERVICE_ACCESS"}, target.Attributes["gcp.memorystore.connect-mode"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.memorystore.auth-enabled"])
	assert.Equal(t, []string{"SERVER_AUTHENTICATION"}, target.Attributes["gcp.memorystore.transit-encryption-mode"])
	assert.Equal(t, []string{"READ_REPLICAS_ENABLED"}, target.Attributes["gcp.memorystore.read-replicas-mode"])
	assert.Equal(t, []string{"2"}, target.Attributes["gcp.memorystore.replica-count"])
	assert.Equal(t, []string{"RDB"}, target.Attributes["gcp.memorystore.persistence-mode"])
	assert.Equal(t, []string{"projects/proj-a/global/networks/default"}, target.Attributes["gcp.memorystore.authorized-network"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.memorystore.label.team"])
}

func TestToRedisTarget_Sparse(t *testing.T) {
	inst := &redispb.Instance{
		Name: "projects/proj-a/locations/europe-west1/instances/sparse",
	}
	target := toRedisTarget(inst, "proj-a")

	assert.Equal(t, "sparse", target.Label)
	assert.NotContains(t, target.Attributes, attrTier)
	assert.NotContains(t, target.Attributes, attrRedisVersion)
	assert.NotContains(t, target.Attributes, "gcp.memorystore.memory-size-gb")
	assert.NotContains(t, target.Attributes, "gcp.memorystore.state")
	assert.NotContains(t, target.Attributes, "gcp.memorystore.replica-count")
	assert.NotContains(t, target.Attributes, "gcp.memorystore.persistence-mode")
	// auth-enabled is written unconditionally
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.memorystore.auth-enabled"])
}

func TestToRedisTarget_MalformedName(t *testing.T) {
	// Name doesn't match the 6-part FQDN — region stays empty, instanceID falls back to raw Name.
	inst := &redispb.Instance{Name: "truncated"}
	target := toRedisTarget(inst, "proj-a")

	assert.Equal(t, "truncated", target.Label)
	assert.Equal(t, []string{"truncated"}, target.Attributes["gcp.memorystore.instance.id"])
	assert.NotContains(t, target.Attributes, attrRegion)
}

func TestDescribeMethods(t *testing.T) {
	d := &redisDiscovery{}
	assert.Equal(t, TargetIDRedisInstance, d.Describe().Id)
	assert.Equal(t, TargetIDRedisInstance, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewRedisDiscovery(t *testing.T) {
	d := NewRedisDiscovery()
	assert.NotNil(t, d)
}

func TestDataProtectionFromString(t *testing.T) {
	m, ok := dataProtectionFromString("FORCE_DATA_LOSS")
	assert.True(t, ok)
	assert.Equal(t, redispb.FailoverInstanceRequest_FORCE_DATA_LOSS, m)

	m, ok = dataProtectionFromString("LIMITED_DATA_LOSS")
	assert.True(t, ok)
	assert.Equal(t, redispb.FailoverInstanceRequest_LIMITED_DATA_LOSS, m)

	// Unknown / typo values are rejected — no silent downgrade to LIMITED.
	m, ok = dataProtectionFromString("force_data_loss")
	assert.False(t, ok)
	assert.Equal(t, redispb.FailoverInstanceRequest_DATA_PROTECTION_MODE_UNSPECIFIED, m)

	m, ok = dataProtectionFromString("")
	assert.False(t, ok)
	assert.Equal(t, redispb.FailoverInstanceRequest_DATA_PROTECTION_MODE_UNSPECIFIED, m)
}
