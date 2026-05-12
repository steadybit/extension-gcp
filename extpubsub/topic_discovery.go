/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extpubsub

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2/apiv1"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
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

type topicDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*topicDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*topicDiscovery)(nil)
)

func NewTopicDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&topicDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *topicDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDTopic,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *topicDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDTopic,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Pub/Sub topic", Other: "Pub/Sub topics"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "gcp.pubsub.topic.message-retention-duration"},
				{Attribute: "gcp.pubsub.topic.kms-key-name"},
				{Attribute: "gcp.project.id"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *topicDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.pubsub.topic.name", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic name", Other: "Pub/Sub topic names"}},
		{Attribute: "gcp.pubsub.topic.message-retention-duration", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic message retention", Other: "Pub/Sub topic message retentions"}},
		{Attribute: "gcp.pubsub.topic.kms-key-name", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic KMS key", Other: "Pub/Sub topic KMS keys"}},
		{Attribute: "gcp.pubsub.topic.message-storage-policy.allowed-persistence-regions", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic allowed region", Other: "Pub/Sub topic allowed regions"}},
		{Attribute: "gcp.pubsub.topic.message-storage-policy.enforce-in-transit", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic enforce in-transit", Other: "Pub/Sub topic enforce in-transit"}},
		{Attribute: "gcp.pubsub.topic.schema-settings.schema", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic schema", Other: "Pub/Sub topic schemas"}},
		{Attribute: "gcp.pubsub.topic.schema-settings.encoding", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic schema encoding", Other: "Pub/Sub topic schema encodings"}},
		{Attribute: "gcp.pubsub.topic.state", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic state", Other: "Pub/Sub topic states"}},
		{Attribute: "gcp.pubsub.topic.satisfies-pzs", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub topic PZS compliance", Other: "Pub/Sub topic PZS compliance"}},
		{Attribute: "gcp.project.id", Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *topicDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := pubsub.NewTopicAdminClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Pub/Sub topic admin client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllTopics(ctx, client, access.ProjectID)
	}, ctx, "pubsub-topic")
}

func getAllTopics(ctx context.Context, client *pubsub.TopicAdminClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.ListTopics(ctx, &pubsubpb.ListTopicsRequest{Project: fmt.Sprintf("projects/%s", projectID)})
	for {
		t, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Pub/Sub topics")
			return nil, err
		}
		targets = append(targets, toTopicTarget(t, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesPubSubTopic), nil
}

func toTopicTarget(t *pubsubpb.Topic, projectID string) discovery_kit_api.Target {
	// t.Name is "projects/<p>/topics/<name>"
	name := t.Name
	if i := strings.LastIndex(t.Name, "/"); i >= 0 {
		name = t.Name[i+1:]
	}

	attributes := make(map[string][]string)
	attributes["gcp.project.id"] = []string{projectID}
	attributes["gcp.pubsub.topic.name"] = []string{name}
	if t.MessageRetentionDuration != nil {
		attributes["gcp.pubsub.topic.message-retention-duration"] = []string{t.MessageRetentionDuration.AsDuration().String()}
	}
	if t.KmsKeyName != "" {
		attributes["gcp.pubsub.topic.kms-key-name"] = []string{t.KmsKeyName}
	}
	if t.MessageStoragePolicy != nil {
		if len(t.MessageStoragePolicy.AllowedPersistenceRegions) > 0 {
			regions := append([]string(nil), t.MessageStoragePolicy.AllowedPersistenceRegions...)
			attributes["gcp.pubsub.topic.message-storage-policy.allowed-persistence-regions"] = regions
		}
		attributes["gcp.pubsub.topic.message-storage-policy.enforce-in-transit"] = []string{strconv.FormatBool(t.MessageStoragePolicy.EnforceInTransit)}
	}
	if t.SchemaSettings != nil {
		if t.SchemaSettings.Schema != "" {
			attributes["gcp.pubsub.topic.schema-settings.schema"] = []string{t.SchemaSettings.Schema}
		}
		if t.SchemaSettings.Encoding != pubsubpb.Encoding_ENCODING_UNSPECIFIED {
			attributes["gcp.pubsub.topic.schema-settings.encoding"] = []string{t.SchemaSettings.Encoding.String()}
		}
	}
	if t.State != pubsubpb.Topic_STATE_UNSPECIFIED {
		attributes["gcp.pubsub.topic.state"] = []string{t.State.String()}
	}
	attributes["gcp.pubsub.topic.satisfies-pzs"] = []string{strconv.FormatBool(t.SatisfiesPzs)}

	for k, v := range t.Labels {
		attributes[fmt.Sprintf("gcp.pubsub.topic.label.%s", strings.ToLower(k))] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         t.Name,
		TargetType: TargetIDTopic,
		Label:      name,
		Attributes: attributes,
	}
}
