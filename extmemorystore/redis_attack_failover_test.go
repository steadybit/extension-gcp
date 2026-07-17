/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmemorystore

import (
	"context"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func redisReq(attrs map[string][]string, cfg map[string]interface{}) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Target: extutil.Ptr(action_kit_api.Target{Attributes: attrs}),
		Config: cfg,
	})
}

var validRedisAttrs = map[string][]string{
	"gcp.project.id":              {"proj-a"},
	"gcp.memorystore.instance.id": {"cache-01"},
	"gcp.memorystore.region":      {"europe-west1"},
	attrTier:                      {"STANDARD_HA"},
}

func TestRedisFailover_Prepare_Success(t *testing.T) {
	a := &redisFailoverAttack{}
	state := RedisFailoverState{}
	_, err := a.Prepare(context.Background(), &state, redisReq(validRedisAttrs, map[string]interface{}{"dataProtectionMode": "LIMITED_DATA_LOSS"}))
	require.NoError(t, err)
	assert.Equal(t, "proj-a", state.ProjectID)
	assert.Equal(t, "cache-01", state.InstanceID)
	assert.Equal(t, "projects/proj-a/locations/europe-west1/instances/cache-01", state.InstanceName)
	assert.Equal(t, "LIMITED_DATA_LOSS", state.DataProtectionMode)
}

func TestRedisFailover_Prepare_ForceMode(t *testing.T) {
	a := &redisFailoverAttack{}
	state := RedisFailoverState{}
	_, err := a.Prepare(context.Background(), &state, redisReq(validRedisAttrs, map[string]interface{}{"dataProtectionMode": "FORCE_DATA_LOSS"}))
	require.NoError(t, err)
	assert.Equal(t, "FORCE_DATA_LOSS", state.DataProtectionMode)
}

func TestRedisFailover_Prepare_MissingRequiredAttr(t *testing.T) {
	for _, drop := range []string{"gcp.project.id", "gcp.memorystore.instance.id", "gcp.memorystore.region"} {
		attrs := map[string][]string{}
		for k, v := range validRedisAttrs {
			if k != drop {
				attrs[k] = v
			}
		}
		a := &redisFailoverAttack{}
		state := RedisFailoverState{}
		_, err := a.Prepare(context.Background(), &state, redisReq(attrs, map[string]interface{}{"dataProtectionMode": "LIMITED_DATA_LOSS"}))
		require.Error(t, err, "dropping %s should fail Prepare", drop)
	}
}

func TestRedisFailover_Prepare_NonHATierRejected(t *testing.T) {
	attrs := map[string][]string{}
	for k, v := range validRedisAttrs {
		attrs[k] = v
	}
	attrs[attrTier] = []string{"BASIC"}

	a := &redisFailoverAttack{}
	state := RedisFailoverState{}
	_, err := a.Prepare(context.Background(), &state, redisReq(attrs, map[string]interface{}{"dataProtectionMode": "LIMITED_DATA_LOSS"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STANDARD_HA")
}

func TestRedisFailover_Prepare_MissingTierRejected(t *testing.T) {
	attrs := map[string][]string{}
	for k, v := range validRedisAttrs {
		if k != attrTier {
			attrs[k] = v
		}
	}

	a := &redisFailoverAttack{}
	state := RedisFailoverState{}
	_, err := a.Prepare(context.Background(), &state, redisReq(attrs, map[string]interface{}{"dataProtectionMode": "LIMITED_DATA_LOSS"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STANDARD_HA")
}

func TestRedisFailover_Prepare_UnknownDataProtectionMode(t *testing.T) {
	a := &redisFailoverAttack{}
	state := RedisFailoverState{}
	_, err := a.Prepare(context.Background(), &state, redisReq(validRedisAttrs, map[string]interface{}{"dataProtectionMode": "force_data_loss"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown dataProtectionMode")
}

func TestRedisFailover_Describe(t *testing.T) {
	a := &redisFailoverAttack{}
	desc := a.Describe()
	assert.Equal(t, RedisFailoverActionId, desc.Id)
	assert.Equal(t, TargetIDRedisInstance, desc.TargetSelection.TargetType)
	assert.Equal(t, RedisFailoverState{}, a.NewEmptyState())
}

func TestRedisFailover_NewAction(t *testing.T) {
	a := NewRedisFailoverAction()
	assert.NotNil(t, a)
}

func TestMustHaveAttr(t *testing.T) {
	attrs := map[string][]string{
		"present": {"value"},
		"empty":   {},
	}
	assert.Equal(t, "value", mustHaveAttr(attrs, "present"))
	assert.Equal(t, "", mustHaveAttr(attrs, "missing"))
	assert.Equal(t, "", mustHaveAttr(attrs, "empty"))
}
