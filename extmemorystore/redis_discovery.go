/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extmemorystore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	redis "cloud.google.com/go/redis/apiv1"
	"cloud.google.com/go/redis/apiv1/redispb"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-gcp/config"
	"github.com/steadybit/extension-gcp/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"google.golang.org/api/iterator"
)

type redisDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*redisDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*redisDiscovery)(nil)
)

func NewRedisDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&redisDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *redisDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDRedisInstance,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *redisDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDRedisInstance,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Memorystore for Redis instance", Other: "Memorystore for Redis instances"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.memorystore.tier"},
				{Attribute: "gcp.memorystore.redis-version"},
				{Attribute: "gcp.memorystore.region"},
				{Attribute: "gcp.project.id"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *redisDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.memorystore.instance.id", Label: discovery_kit_api.PluralLabel{One: "Memorystore instance ID", Other: "Memorystore instance IDs"}},
		{Attribute: "gcp.memorystore.tier", Label: discovery_kit_api.PluralLabel{One: "Memorystore tier", Other: "Memorystore tiers"}},
		{Attribute: "gcp.memorystore.redis-version", Label: discovery_kit_api.PluralLabel{One: "Memorystore Redis version", Other: "Memorystore Redis versions"}},
		{Attribute: "gcp.memorystore.region", Label: discovery_kit_api.PluralLabel{One: "Memorystore region", Other: "Memorystore regions"}},
		{Attribute: "gcp.memorystore.location-id", Label: discovery_kit_api.PluralLabel{One: "Memorystore location", Other: "Memorystore locations"}},
		{Attribute: "gcp.memorystore.alternative-location-id", Label: discovery_kit_api.PluralLabel{One: "Memorystore alternative location", Other: "Memorystore alternative locations"}},
		{Attribute: "gcp.memorystore.memory-size-gb", Label: discovery_kit_api.PluralLabel{One: "Memorystore memory size (GiB)", Other: "Memorystore memory sizes (GiB)"}},
		{Attribute: "gcp.memorystore.state", Label: discovery_kit_api.PluralLabel{One: "Memorystore state", Other: "Memorystore states"}},
		{Attribute: "gcp.memorystore.connect-mode", Label: discovery_kit_api.PluralLabel{One: "Memorystore connect mode", Other: "Memorystore connect modes"}},
		{Attribute: "gcp.memorystore.auth-enabled", Label: discovery_kit_api.PluralLabel{One: "Memorystore AUTH enabled", Other: "Memorystore AUTH enabled"}},
		{Attribute: "gcp.memorystore.transit-encryption-mode", Label: discovery_kit_api.PluralLabel{One: "Memorystore transit encryption", Other: "Memorystore transit encryption modes"}},
		{Attribute: "gcp.memorystore.read-replicas-mode", Label: discovery_kit_api.PluralLabel{One: "Memorystore read replicas mode", Other: "Memorystore read replicas modes"}},
		{Attribute: "gcp.memorystore.replica-count", Label: discovery_kit_api.PluralLabel{One: "Memorystore replica count", Other: "Memorystore replica counts"}},
		{Attribute: "gcp.memorystore.persistence-mode", Label: discovery_kit_api.PluralLabel{One: "Memorystore persistence mode", Other: "Memorystore persistence modes"}},
		{Attribute: "gcp.memorystore.authorized-network", Label: discovery_kit_api.PluralLabel{One: "Memorystore authorized network", Other: "Memorystore authorized networks"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *redisDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := redis.NewCloudRedisClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create CloudRedis client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllRedisInstances(ctx, client, access.ProjectID)
	}, ctx, "memorystore-redis")
}

func getAllRedisInstances(ctx context.Context, client *redis.CloudRedisClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	// '-' wildcard returns instances across all regions of the project.
	it := client.ListInstances(ctx, &redispb.ListInstancesRequest{Parent: fmt.Sprintf("projects/%s/locations/-", projectID)})
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Memorystore Redis instances")
			return nil, err
		}
		targets = append(targets, toRedisTarget(inst, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesMemorystoreRedis), nil
}

func toRedisTarget(inst *redispb.Instance, projectID string) discovery_kit_api.Target {
	// inst.Name is the fully-qualified resource name: projects/<p>/locations/<region>/instances/<id>.
	region := ""
	instanceID := inst.Name
	if parts := strings.Split(inst.Name, "/"); len(parts) >= 6 {
		region = parts[3]
		instanceID = parts[5]
	}

	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.memorystore.instance.id"] = []string{instanceID}
	if region != "" {
		attributes["gcp.memorystore.region"] = []string{region}
	}
	if inst.Tier != redispb.Instance_TIER_UNSPECIFIED {
		attributes["gcp.memorystore.tier"] = []string{inst.Tier.String()}
	}
	if inst.RedisVersion != "" {
		attributes["gcp.memorystore.redis-version"] = []string{inst.RedisVersion}
	}
	if inst.LocationId != "" {
		attributes["gcp.memorystore.location-id"] = []string{inst.LocationId}
	}
	if inst.AlternativeLocationId != "" {
		attributes["gcp.memorystore.alternative-location-id"] = []string{inst.AlternativeLocationId}
	}
	if inst.MemorySizeGb > 0 {
		attributes["gcp.memorystore.memory-size-gb"] = []string{strconv.Itoa(int(inst.MemorySizeGb))}
	}
	if inst.State != redispb.Instance_STATE_UNSPECIFIED {
		attributes["gcp.memorystore.state"] = []string{inst.State.String()}
	}
	if inst.ConnectMode != redispb.Instance_CONNECT_MODE_UNSPECIFIED {
		attributes["gcp.memorystore.connect-mode"] = []string{inst.ConnectMode.String()}
	}
	attributes["gcp.memorystore.auth-enabled"] = []string{strconv.FormatBool(inst.AuthEnabled)}
	if inst.TransitEncryptionMode != redispb.Instance_TRANSIT_ENCRYPTION_MODE_UNSPECIFIED {
		attributes["gcp.memorystore.transit-encryption-mode"] = []string{inst.TransitEncryptionMode.String()}
	}
	if inst.ReadReplicasMode != redispb.Instance_READ_REPLICAS_MODE_UNSPECIFIED {
		attributes["gcp.memorystore.read-replicas-mode"] = []string{inst.ReadReplicasMode.String()}
	}
	if inst.ReplicaCount > 0 {
		attributes["gcp.memorystore.replica-count"] = []string{strconv.Itoa(int(inst.ReplicaCount))}
	}
	if inst.PersistenceConfig != nil && inst.PersistenceConfig.PersistenceMode != redispb.PersistenceConfig_PERSISTENCE_MODE_UNSPECIFIED {
		attributes["gcp.memorystore.persistence-mode"] = []string{inst.PersistenceConfig.PersistenceMode.String()}
	}
	if inst.AuthorizedNetwork != "" {
		attributes["gcp.memorystore.authorized-network"] = []string{inst.AuthorizedNetwork}
	}
	for k, v := range inst.Labels {
		attributes[fmt.Sprintf("gcp.memorystore.label.%s", strings.ToLower(k))] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         inst.Name,
		TargetType: TargetIDRedisInstance,
		Label:      instanceID,
		Attributes: attributes,
	}
}
