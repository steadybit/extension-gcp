/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extpubsub

const (
	TargetIDTopic        = "com.steadybit.extension_gcp.pubsub.topic"
	TargetIDSubscription = "com.steadybit.extension_gcp.pubsub.subscription"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB2aWV3Qm94PSIwIDAgNTEyIDUxMiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aCBkPSJNMzQ2LjY4MywxMTIuNjExbC0uMDM5LTc1LjE3MWMtLjAwNS04LjgzNC03LjE2Ny0xNS45OTItMTYtMTUuOTkyaC0uMDA4Yy04LjgzNy4wMDQtMTUuOTk3LDcuMTcyLTE1Ljk5MiwxNi4wMDhsLjAzOSw3NS4xNTVIMTE4Yy0xNy42NzMsMC0zMiwxNC4zMjctMzIsMzJ2MTExLjg2NWMwLDg5LjA4NSw3Ni4yNjIsMTYxLjU2MiwxNzAsMTYxLjU2MnMxNzAtNzIuNDc3LDE3MC0xNjEuNTYydi0xMTEuODY1YzAtMTcuNjczLTE0LjMyNy0zMi0zMi0zMmgtNDcuMzE3Wk0zOTQsMjU2LjQ3N2MwLDcxLjQ0LTYxLjkwNiwxMjkuNTYyLTEzOCwxMjkuNTYycy0xMzgtNTguMTIxLTEzOC0xMjkuNTYydi0xMTEuODY1aDI3NnYxMTEuODY1WiIgZmlsbD0iY3VycmVudENvbG9yIiAvPgogIDxwYXRoIGQ9Ik0zMTQuNjQ0LDE0NC42MTF2LTMyaC0xMTcuMjk0bC0uMDM5LTc1LjE3MWMtLjAwNS04LjgzNC03LjE2Ny0xNS45OTItMTYtMTUuOTkyaC0uMDA4Yy04LjgzNy4wMDQtMTUuOTk3LDcuMTcyLTE1Ljk5MiwxNi4wMDhsLjAzOSw3NS4xNTVoLTQ3LjM1Yy0xNy42NzMsMC0zMiwxNC4zMjctMzIsMzJ2MTExLjg2NWMwLDgzLjk1OSw2Ny43NDEsMTUzLjE1NSwxNTQuMDA3LDE2MC44NDF2NTYuNjc5YzAsOC42MTYsNi42MjEsMTYuMDI5LDE1LjIyOCwxNi40MzMsOS4xODguNDMyLDE2Ljc3Mi02Ljg4OSwxNi43NzItMTUuOTgydi04OS4yNzdjLTYuNTk0LjYyNy04LjAxNC42NTItMTYsLjg2N2gtLjAwN2MtNzYuMDk0LDAtMTM4LTU4LjEyMS0xMzgtMTI5LjU2MnYtMTExLjg2NWgxOTYuNjQ0LDBaIiBmaWxsPSJjdXJyZW50Q29sb3IiIC8+Cjwvc3ZnPg=="

	// Attribute names extracted per Sonar go:S1192. Shared across topic and
	// subscription discovery files in this package.
	attrProjectID                      = "gcp.project.id"
	attrSubscriptionTopic              = "gcp.pubsub.subscription.topic"
	attrSubscriptionDeliveryType       = "gcp.pubsub.subscription.delivery-type"
	attrSubscriptionAckDeadlineSeconds = "gcp.pubsub.subscription.ack-deadline-seconds"
	attrTopicMessageRetentionDuration  = "gcp.pubsub.topic.message-retention-duration"
	attrTopicKmsKeyName                = "gcp.pubsub.topic.kms-key-name"
)
