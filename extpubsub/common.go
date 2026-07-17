/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extpubsub

const (
	TargetIDTopic        = "com.steadybit.extension_gcp.pubsub.topic"
	TargetIDSubscription = "com.steadybit.extension_gcp.pubsub.subscription"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djJINFY0em0wIDdoMTZ2Mkg0di0yem0wIDdoMTZ2Mkg0di0yeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"

	// Attribute names extracted per Sonar go:S1192. Shared across topic and
	// subscription discovery files in this package.
	attrProjectID                      = "gcp.project.id"
	attrSubscriptionTopic              = "gcp.pubsub.subscription.topic"
	attrSubscriptionDeliveryType       = "gcp.pubsub.subscription.delivery-type"
	attrSubscriptionAckDeadlineSeconds = "gcp.pubsub.subscription.ack-deadline-seconds"
	attrTopicMessageRetentionDuration  = "gcp.pubsub.topic.message-retention-duration"
	attrTopicKmsKeyName                = "gcp.pubsub.topic.kms-key-name"
)
