/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extpubsub

import (
	"context"
	"fmt"
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

type subscriptionDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*subscriptionDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*subscriptionDiscovery)(nil)
)

func NewSubscriptionDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&subscriptionDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *subscriptionDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDSubscription,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *subscriptionDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDSubscription,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Pub/Sub subscription", Other: "Pub/Sub subscriptions"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: attrSubscriptionTopic},
				{Attribute: attrSubscriptionDeliveryType},
				{Attribute: attrSubscriptionAckDeadlineSeconds},
				{Attribute: attrProjectID},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *subscriptionDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "gcp.pubsub.subscription.name", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription name", Other: "Pub/Sub subscription names"}},
		{Attribute: attrSubscriptionTopic, Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription topic", Other: "Pub/Sub subscription topics"}},
		{Attribute: attrSubscriptionDeliveryType, Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription delivery type", Other: "Pub/Sub subscription delivery types"}},
		{Attribute: attrSubscriptionAckDeadlineSeconds, Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription ack deadline", Other: "Pub/Sub subscription ack deadlines"}},
		{Attribute: "gcp.pubsub.subscription.message-retention-duration", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription message retention", Other: "Pub/Sub subscription message retentions"}},
		{Attribute: "gcp.pubsub.subscription.retain-acked-messages", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription retain acked", Other: "Pub/Sub subscription retain acked"}},
		{Attribute: "gcp.pubsub.subscription.enable-message-ordering", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription message ordering", Other: "Pub/Sub subscription message ordering"}},
		{Attribute: "gcp.pubsub.subscription.enable-exactly-once-delivery", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription exactly-once delivery", Other: "Pub/Sub subscription exactly-once delivery"}},
		{Attribute: "gcp.pubsub.subscription.filter", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription filter", Other: "Pub/Sub subscription filters"}},
		{Attribute: "gcp.pubsub.subscription.detached", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription detached", Other: "Pub/Sub subscription detached"}},
		{Attribute: "gcp.pubsub.subscription.state", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription state", Other: "Pub/Sub subscription states"}},
		{Attribute: "gcp.pubsub.subscription.push-config.endpoint", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription push endpoint", Other: "Pub/Sub subscription push endpoints"}},
		{Attribute: "gcp.pubsub.subscription.dead-letter-policy.topic", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription dead-letter topic", Other: "Pub/Sub subscription dead-letter topics"}},
		{Attribute: "gcp.pubsub.subscription.dead-letter-policy.max-delivery-attempts", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription max delivery attempts", Other: "Pub/Sub subscription max delivery attempts"}},
		{Attribute: "gcp.pubsub.subscription.retry-policy.minimum-backoff", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription retry min backoff", Other: "Pub/Sub subscription retry min backoffs"}},
		{Attribute: "gcp.pubsub.subscription.retry-policy.maximum-backoff", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription retry max backoff", Other: "Pub/Sub subscription retry max backoffs"}},
		{Attribute: "gcp.pubsub.subscription.expiration-policy.ttl", Label: discovery_kit_api.PluralLabel{One: "Pub/Sub subscription expiration TTL", Other: "Pub/Sub subscription expiration TTLs"}},
		{Attribute: attrProjectID, Label: discovery_kit_api.PluralLabel{One: "GCP project ID", Other: "GCP project IDs"}},
	}
}

func (d *subscriptionDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredGcpAccess(func(access *utils.GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		client, err := pubsub.NewSubscriptionAdminClient(ctx, access.ClientOptions...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Pub/Sub subscription admin client for project '%s': %w", access.ProjectID, err)
		}
		defer func() { _ = client.Close() }()
		return getAllSubscriptions(ctx, client, access.ProjectID)
	}, ctx, "pubsub-subscription")
}

func getAllSubscriptions(ctx context.Context, client *pubsub.SubscriptionAdminClient, projectID string) ([]discovery_kit_api.Target, error) {
	targets := make([]discovery_kit_api.Target, 0)
	it := client.ListSubscriptions(ctx, &pubsubpb.ListSubscriptionsRequest{Project: fmt.Sprintf("projects/%s", projectID)})
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Warn().Err(err).Str("project", projectID).Msg("Failed to list Pub/Sub subscriptions")
			return nil, err
		}
		targets = append(targets, toSubscriptionTarget(s, projectID))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesPubSubSubscription), nil
}

func toSubscriptionTarget(s *pubsubpb.Subscription, projectID string) discovery_kit_api.Target {
	// s.Name is "projects/<p>/subscriptions/<name>"
	name := s.Name
	if i := strings.LastIndex(s.Name, "/"); i >= 0 {
		name = s.Name[i+1:]
	}

	attributes := make(map[string][]string)
	attributes[attrProjectID] = []string{projectID}
	attributes["gcp.pubsub.subscription.name"] = []string{name}
	utils.SetStr(attributes, attrSubscriptionTopic, s.Topic)
	attributes[attrSubscriptionDeliveryType] = []string{deliveryType(s)}
	utils.SetInt64IfPositive(attributes, attrSubscriptionAckDeadlineSeconds, int64(s.AckDeadlineSeconds))
	addMessageRetentionAttr(attributes, s)
	utils.SetBool(attributes, "gcp.pubsub.subscription.retain-acked-messages", s.RetainAckedMessages)
	utils.SetBool(attributes, "gcp.pubsub.subscription.enable-message-ordering", s.EnableMessageOrdering)
	utils.SetBool(attributes, "gcp.pubsub.subscription.enable-exactly-once-delivery", s.EnableExactlyOnceDelivery)
	utils.SetBool(attributes, "gcp.pubsub.subscription.detached", s.Detached)
	utils.SetStr(attributes, "gcp.pubsub.subscription.filter", s.Filter)
	addStateAttr(attributes, s.State)
	addPushConfigAttrs(attributes, s.PushConfig)
	addDeadLetterPolicyAttrs(attributes, s.DeadLetterPolicy)
	addRetryPolicyAttrs(attributes, s.RetryPolicy)
	addExpirationPolicyAttrs(attributes, s.ExpirationPolicy)

	for k, v := range s.Labels {
		utils.SetStr(attributes, fmt.Sprintf("gcp.pubsub.subscription.label.%s", strings.ToLower(k)), v)
	}

	return discovery_kit_api.Target{
		Id:         s.Name,
		TargetType: TargetIDSubscription,
		Label:      name,
		Attributes: attributes,
	}
}

func addMessageRetentionAttr(attrs map[string][]string, s *pubsubpb.Subscription) {
	if s.MessageRetentionDuration == nil {
		return
	}
	attrs["gcp.pubsub.subscription.message-retention-duration"] = []string{s.MessageRetentionDuration.AsDuration().String()}
}

func addStateAttr(attrs map[string][]string, state pubsubpb.Subscription_State) {
	if state == pubsubpb.Subscription_STATE_UNSPECIFIED {
		return
	}
	attrs["gcp.pubsub.subscription.state"] = []string{state.String()}
}

func addPushConfigAttrs(attrs map[string][]string, p *pubsubpb.PushConfig) {
	if p == nil {
		return
	}
	utils.SetStr(attrs, "gcp.pubsub.subscription.push-config.endpoint", p.PushEndpoint)
}

func addDeadLetterPolicyAttrs(attrs map[string][]string, d *pubsubpb.DeadLetterPolicy) {
	if d == nil {
		return
	}
	utils.SetStr(attrs, "gcp.pubsub.subscription.dead-letter-policy.topic", d.DeadLetterTopic)
	utils.SetInt64IfPositive(attrs, "gcp.pubsub.subscription.dead-letter-policy.max-delivery-attempts", int64(d.MaxDeliveryAttempts))
}

func addRetryPolicyAttrs(attrs map[string][]string, r *pubsubpb.RetryPolicy) {
	if r == nil {
		return
	}
	if r.MinimumBackoff != nil {
		attrs["gcp.pubsub.subscription.retry-policy.minimum-backoff"] = []string{r.MinimumBackoff.AsDuration().String()}
	}
	if r.MaximumBackoff != nil {
		attrs["gcp.pubsub.subscription.retry-policy.maximum-backoff"] = []string{r.MaximumBackoff.AsDuration().String()}
	}
}

func addExpirationPolicyAttrs(attrs map[string][]string, e *pubsubpb.ExpirationPolicy) {
	if e == nil {
		return
	}
	if e.Ttl != nil {
		attrs["gcp.pubsub.subscription.expiration-policy.ttl"] = []string{e.Ttl.AsDuration().String()}
	}
}

// deliveryType returns "push", "bigquery", "cloud-storage", "bigtable", or "pull" (default for pull subscriptions).
func deliveryType(s *pubsubpb.Subscription) string {
	switch {
	case s.PushConfig != nil && s.PushConfig.PushEndpoint != "":
		return "push"
	case s.BigqueryConfig != nil:
		return "bigquery"
	case s.CloudStorageConfig != nil:
		return "cloud-storage"
	case s.BigtableConfig != nil:
		return "bigtable"
	default:
		return "pull"
	}
}
