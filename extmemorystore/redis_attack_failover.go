/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmemorystore

import (
	"context"
	"fmt"

	redis "cloud.google.com/go/redis/apiv1"
	"cloud.google.com/go/redis/apiv1/redispb"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-gcp/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

// RedisFailoverState is enough to trigger a failover. Only STANDARD_HA tier instances support failover.
// The attack is instantaneous and is not reversible: it exercises the same code path as a real zonal
// outage on the primary node.
type RedisFailoverState struct {
	ProjectID          string
	InstanceName       string // fully-qualified: projects/<p>/locations/<region>/instances/<id>
	InstanceID         string
	DataProtectionMode string
}

type redisFailoverAttack struct {
	clientProvider func(ctx context.Context, projectID string) (*redis.CloudRedisClient, func(), error)
}

var _ action_kit_sdk.Action[RedisFailoverState] = (*redisFailoverAttack)(nil)

func NewRedisFailoverAction() action_kit_sdk.Action[RedisFailoverState] {
	return &redisFailoverAttack{
		clientProvider: func(ctx context.Context, projectID string) (*redis.CloudRedisClient, func(), error) {
			access, err := utils.GetGcpAccess(projectID)
			if err != nil {
				return nil, nil, err
			}
			c, err := redis.NewCloudRedisClient(ctx, access.ClientOptions...)
			if err != nil {
				return nil, nil, err
			}
			return c, func() { _ = c.Close() }, nil
		},
	}
}

func (a *redisFailoverAttack) NewEmptyState() RedisFailoverState { return RedisFailoverState{} }

func (a *redisFailoverAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    RedisFailoverActionId,
		Label: "Trigger Memorystore for Redis failover",
		Description: "Triggers a failover from the primary node to a replica for a STANDARD_HA tier Memorystore for Redis instance. " +
			"Validates that connection-pool retry / reconnect logic survives the brief read/write interruption. " +
			"This is not reversible — it exercises the same code path as a real primary-node outage. " +
			"FORCE_DATA_LOSS may drop in-flight writes that have not yet been replicated.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDRedisInstance,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by instance ID",
					Description: extutil.Ptr("Find Memorystore Redis instance by ID"),
					Query:       "gcp.memorystore.instance.id=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Google Cloud"),
		Category:    extutil.Ptr("Memorystore"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "dataProtectionMode",
				Label:        "Data protection mode",
				Description:  extutil.Ptr("LIMITED_DATA_LOSS waits for in-flight writes to flush to the replica before failing over; FORCE_DATA_LOSS fails over immediately and may lose recent writes."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: extutil.Ptr("LIMITED_DATA_LOSS"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{Label: "Limited data loss (graceful)", Value: "LIMITED_DATA_LOSS"},
					action_kit_api.ExplicitParameterOption{Label: "Force data loss (immediate)", Value: "FORCE_DATA_LOSS"},
				}),
			},
		},
	}
}

// dataProtectionFromString maps the parameter value to the SDK enum. Defaults to LIMITED_DATA_LOSS.
func dataProtectionFromString(s string) redispb.FailoverInstanceRequest_DataProtectionMode {
	switch s {
	case "FORCE_DATA_LOSS":
		return redispb.FailoverInstanceRequest_FORCE_DATA_LOSS
	default:
		return redispb.FailoverInstanceRequest_LIMITED_DATA_LOSS
	}
}

func (a *redisFailoverAttack) Prepare(_ context.Context, state *RedisFailoverState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.ProjectID = mustHaveAttr(request.Target.Attributes, "gcp.project.id")
	state.InstanceID = mustHaveAttr(request.Target.Attributes, "gcp.memorystore.instance.id")
	region := mustHaveAttr(request.Target.Attributes, "gcp.memorystore.region")
	if state.ProjectID == "" || state.InstanceID == "" || region == "" {
		return nil, extension_kit.ToError("Target is missing one of: gcp.project.id, gcp.memorystore.instance.id, gcp.memorystore.region", nil)
	}
	if tier := request.Target.Attributes["gcp.memorystore.tier"]; len(tier) == 0 || tier[0] != "STANDARD_HA" {
		return nil, extension_kit.ToError(fmt.Sprintf("Memorystore instance %s is not STANDARD_HA; failover requires a high-availability instance", state.InstanceID), nil)
	}
	state.InstanceName = fmt.Sprintf("projects/%s/locations/%s/instances/%s", state.ProjectID, region, state.InstanceID)
	state.DataProtectionMode = extutil.ToString(request.Config["dataProtectionMode"])
	return nil, nil
}

func (a *redisFailoverAttack) Start(ctx context.Context, state *RedisFailoverState) (*action_kit_api.StartResult, error) {
	client, closer, err := a.clientProvider(ctx, state.ProjectID)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize CloudRedis client for project %s", state.ProjectID), err)
	}
	defer closer()
	mode := dataProtectionFromString(state.DataProtectionMode)
	// FailoverInstance returns a long-running operation; we don't wait — chaos = fire-and-forget.
	_, err = client.FailoverInstance(ctx, &redispb.FailoverInstanceRequest{
		Name:               state.InstanceName,
		DataProtectionMode: mode,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to trigger Memorystore failover for %s", state.InstanceID), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Failover triggered for Memorystore Redis instance %s", state.InstanceID),
		}}),
	}, nil
}

func mustHaveAttr(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}
