/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

const (
	TargetIDCluster                    = "com.steadybit.extension_gcp.gke.cluster"
	TargetIDNodePool                   = "com.steadybit.extension_gcp.gke.nodepool"
	NodePoolTerminateInstancesActionId = "com.steadybit.extension_gcp.gke.nodepool.terminate-instances"
	targetIcon                         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmwxMCA1djEwbC0xMCA1LTEwLTVWN2wxMC01eiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"

	// Attribute names extracted per Sonar go:S1192. Shared across cluster,
	// nodepool discovery, and the terminate-instances attack.
	attrProjectID                          = "gcp.project.id"
	attrK8sClusterName                     = "k8s.cluster-name"
	attrClusterName                        = "gcp.gke.cluster.name"
	attrClusterLocation                    = "gcp.gke.cluster.location"
	attrClusterLocationType                = "gcp.gke.cluster.location-type"
	attrClusterKubernetesVersion           = "gcp.gke.cluster.kubernetes-version"
	attrClusterReleaseChannel              = "gcp.gke.cluster.release-channel"
	attrClusterPrivateCluster              = "gcp.gke.cluster.private-cluster"
	attrClusterMasterAuthorizedNetsEnabled = "gcp.gke.cluster.master-authorized-networks-enabled"
	attrClusterMasterAuthorizedNetsCidrs   = "gcp.gke.cluster.master-authorized-networks-cidrs"
	attrClusterApiServerOpenToInternet     = "gcp.gke.cluster.api-server-open-to-internet"
	attrClusterNetwork                     = "gcp.gke.cluster.network"
	attrClusterSubnetwork                  = "gcp.gke.cluster.subnetwork"
	attrClusterWorkloadIdentityEnabled     = "gcp.gke.cluster.workload-identity-enabled"
	attrClusterShieldedNodesEnabled        = "gcp.gke.cluster.shielded-nodes-enabled"
	attrClusterBinaryAuthEvalMode          = "gcp.gke.cluster.binary-authorization-evaluation-mode"
	attrClusterLoggingService              = "gcp.gke.cluster.logging-service"
	attrClusterMonitoringService           = "gcp.gke.cluster.monitoring-service"
	attrNodePoolKubernetesVersion          = "gcp.gke.nodepool.kubernetes-version"
	attrNodePoolMachineType                = "gcp.gke.nodepool.machine-type"
	attrNodePoolAutoscalingEnabled         = "gcp.gke.nodepool.autoscaling.enabled"
)
