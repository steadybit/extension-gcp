/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extpubsub

import (
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestToTopicTarget_Populated(t *testing.T) {
	topic := &pubsubpb.Topic{
		Name:                     "projects/proj-a/topics/orders",
		MessageRetentionDuration: durationpb.New(24 * time.Hour),
		KmsKeyName:               "projects/proj-a/locations/global/keyRings/kr/cryptoKeys/k",
		MessageStoragePolicy: &pubsubpb.MessageStoragePolicy{
			AllowedPersistenceRegions: []string{"europe-west1", "us-central1"},
			EnforceInTransit:          true,
		},
		SchemaSettings: &pubsubpb.SchemaSettings{
			Schema:   "projects/proj-a/schemas/order",
			Encoding: pubsubpb.Encoding_JSON,
		},
		State:        pubsubpb.Topic_ACTIVE,
		SatisfiesPzs: true,
		Labels:       map[string]string{"team": "core"},
	}

	target := toTopicTarget(topic, "proj-a")

	assert.Equal(t, TargetIDTopic, target.TargetType)
	assert.Equal(t, "orders", target.Label)
	assert.Equal(t, topic.Name, target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"orders"}, target.Attributes["gcp.pubsub.topic.name"])
	assert.Equal(t, []string{"24h0m0s"}, target.Attributes[attrTopicMessageRetentionDuration])
	assert.Equal(t, []string{"projects/proj-a/locations/global/keyRings/kr/cryptoKeys/k"}, target.Attributes[attrTopicKmsKeyName])
	assert.Equal(t, []string{"europe-west1", "us-central1"}, target.Attributes["gcp.pubsub.topic.message-storage-policy.allowed-persistence-regions"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.pubsub.topic.message-storage-policy.enforce-in-transit"])
	assert.Equal(t, []string{"projects/proj-a/schemas/order"}, target.Attributes["gcp.pubsub.topic.schema-settings.schema"])
	assert.Equal(t, []string{"JSON"}, target.Attributes["gcp.pubsub.topic.schema-settings.encoding"])
	assert.Equal(t, []string{"ACTIVE"}, target.Attributes["gcp.pubsub.topic.state"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.pubsub.topic.satisfies-pzs"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.pubsub.topic.label.team"])
}

func TestToTopicTarget_Sparse(t *testing.T) {
	topic := &pubsubpb.Topic{Name: "projects/proj-a/topics/sparse"}
	target := toTopicTarget(topic, "proj-a")
	assert.Equal(t, "sparse", target.Label)
	assert.NotContains(t, target.Attributes, attrTopicMessageRetentionDuration)
	assert.NotContains(t, target.Attributes, attrTopicKmsKeyName)
	// satisfies-pzs always emitted
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.pubsub.topic.satisfies-pzs"])
}

func TestToSubscriptionTarget_Push(t *testing.T) {
	sub := &pubsubpb.Subscription{
		Name:                      "projects/proj-a/subscriptions/orders-sub",
		Topic:                     "projects/proj-a/topics/orders",
		AckDeadlineSeconds:        20,
		MessageRetentionDuration:  durationpb.New(7 * 24 * time.Hour),
		RetainAckedMessages:       false,
		EnableMessageOrdering:     true,
		EnableExactlyOnceDelivery: true,
		Detached:                  false,
		Filter:                    "attributes.type = \"paid\"",
		State:                     pubsubpb.Subscription_ACTIVE,
		PushConfig:                &pubsubpb.PushConfig{PushEndpoint: "https://example.com/webhook"},
		DeadLetterPolicy: &pubsubpb.DeadLetterPolicy{
			DeadLetterTopic:     "projects/proj-a/topics/dead",
			MaxDeliveryAttempts: 5,
		},
		RetryPolicy: &pubsubpb.RetryPolicy{
			MinimumBackoff: durationpb.New(1 * time.Second),
			MaximumBackoff: durationpb.New(60 * time.Second),
		},
		ExpirationPolicy: &pubsubpb.ExpirationPolicy{Ttl: durationpb.New(31 * 24 * time.Hour)},
		Labels:           map[string]string{"team": "core"},
	}

	target := toSubscriptionTarget(sub, "proj-a")

	assert.Equal(t, TargetIDSubscription, target.TargetType)
	assert.Equal(t, "orders-sub", target.Label)
	assert.Equal(t, sub.Name, target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"orders-sub"}, target.Attributes["gcp.pubsub.subscription.name"])
	assert.Equal(t, []string{"projects/proj-a/topics/orders"}, target.Attributes[attrSubscriptionTopic])
	assert.Equal(t, []string{"push"}, target.Attributes[attrSubscriptionDeliveryType])
	assert.Equal(t, []string{"20"}, target.Attributes[attrSubscriptionAckDeadlineSeconds])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.pubsub.subscription.enable-message-ordering"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.pubsub.subscription.enable-exactly-once-delivery"])
	assert.Equal(t, []string{"attributes.type = \"paid\""}, target.Attributes["gcp.pubsub.subscription.filter"])
	assert.Equal(t, []string{"ACTIVE"}, target.Attributes["gcp.pubsub.subscription.state"])
	assert.Equal(t, []string{"https://example.com/webhook"}, target.Attributes["gcp.pubsub.subscription.push-config.endpoint"])
	assert.Equal(t, []string{"projects/proj-a/topics/dead"}, target.Attributes["gcp.pubsub.subscription.dead-letter-policy.topic"])
	assert.Equal(t, []string{"5"}, target.Attributes["gcp.pubsub.subscription.dead-letter-policy.max-delivery-attempts"])
	assert.Equal(t, []string{"1s"}, target.Attributes["gcp.pubsub.subscription.retry-policy.minimum-backoff"])
	assert.Equal(t, []string{"1m0s"}, target.Attributes["gcp.pubsub.subscription.retry-policy.maximum-backoff"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.pubsub.subscription.label.team"])
}

func TestToSubscriptionTarget_DeliveryTypeMatrix(t *testing.T) {
	base := &pubsubpb.Subscription{Name: "projects/p/subscriptions/s"}

	// Pull (default)
	assert.Equal(t, "pull", deliveryType(base))

	// Push
	base.PushConfig = &pubsubpb.PushConfig{PushEndpoint: "https://x"}
	assert.Equal(t, "push", deliveryType(base))
	base.PushConfig = nil

	// BigQuery
	base.BigqueryConfig = &pubsubpb.BigQueryConfig{Table: "t"}
	assert.Equal(t, "bigquery", deliveryType(base))
	base.BigqueryConfig = nil

	// Cloud Storage
	base.CloudStorageConfig = &pubsubpb.CloudStorageConfig{Bucket: "b"}
	assert.Equal(t, "cloud-storage", deliveryType(base))
	base.CloudStorageConfig = nil

	// Bigtable
	base.BigtableConfig = &pubsubpb.BigtableConfig{Table: "t"}
	assert.Equal(t, "bigtable", deliveryType(base))
}

func TestToSubscriptionTarget_Sparse(t *testing.T) {
	sub := &pubsubpb.Subscription{Name: "projects/proj-a/subscriptions/sparse"}
	target := toSubscriptionTarget(sub, "proj-a")
	assert.Equal(t, "sparse", target.Label)
	assert.Equal(t, []string{"pull"}, target.Attributes[attrSubscriptionDeliveryType])
	// Always-emit booleans still show up:
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.pubsub.subscription.retain-acked-messages"])
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.pubsub.subscription.enable-message-ordering"])
	assert.NotContains(t, target.Attributes, attrSubscriptionAckDeadlineSeconds)
}

func TestPubsubDescribeMethods(t *testing.T) {
	td := &topicDiscovery{}
	assert.Equal(t, TargetIDTopic, td.Describe().Id)
	assert.Equal(t, TargetIDTopic, td.DescribeTarget().Id)
	assert.NotEmpty(t, td.DescribeAttributes())

	sd := &subscriptionDiscovery{}
	assert.Equal(t, TargetIDSubscription, sd.Describe().Id)
	assert.Equal(t, TargetIDSubscription, sd.DescribeTarget().Id)
	assert.NotEmpty(t, sd.DescribeAttributes())
}

func TestNewPubsubDiscoveries(t *testing.T) {
	assert.NotNil(t, NewTopicDiscovery())
	assert.NotNil(t, NewSubscriptionDiscovery())
}
