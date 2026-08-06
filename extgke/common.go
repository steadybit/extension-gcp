/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extgke

const (
	TargetIDCluster                    = "com.steadybit.extension_gcp.gke.cluster"
	TargetIDNodePool                   = "com.steadybit.extension_gcp.gke.nodepool"
	NodePoolTerminateInstancesActionId = "com.steadybit.extension_gcp.gke.nodepool.terminate-instances"
	targetIcon                         = "data:image/svg+xml;base64,PHN2ZyB2aWV3Qm94PSIwIDAgNTEyIDUxMiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aCBkPSJNMjU2LDQ1OWMtMi43LDAtNS40LS43LTcuOC0ybC0xNjYuMi05My41Yy01LTIuOC04LjItOC4yLTguMi0xNHYtMTg3YzAtNS44LDMuMS0xMS4xLDguMi0xMy45TDI0OC4yLDU1YzQuOS0yLjcsMTAuOC0yLjcsMTUuNywwbDE2Ni4yLDkzLjVjNSwyLjgsOC4yLDguMiw4LjIsMTMuOXYxODdjMCw1LjgtMy4xLDExLjEtOC4yLDE0bC0xNjYuMiw5My41Yy0yLjQsMS40LTUuMSwyLTcuOCwyaDBaTTEwNS44LDM0MC4xbDE1MC4yLDg0LjUsMTUwLjItODQuNXYtMTY4LjNsLTE1MC4yLTg0LjUtMTUwLjIsODQuNXYxNjguM1pNNDIyLjIsMzQ5LjVoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNODkuOCwxNzguNWMtNS42LDAtMTEtMi45LTE0LTguMi00LjMtNy43LTEuNi0xNy41LDYuMS0yMS44TDI0OC4yLDU1YzcuNy00LjMsMTcuNS0xLjYsMjEuOCw2LjEsNC4zLDcuNywxLjYsMTcuNS02LjEsMjEuOGwtMTY2LjIsOTMuNWMtMi41LDEuNC01LjIsMi4xLTcuOCwyLjFoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNNDIyLjIsMTc4LjVjLTIuNywwLTUuNC0uNy03LjgtMi4xbC0xNjYuMi05My41Yy03LjctNC4zLTEwLjQtMTQuMS02LjEtMjEuOCw0LjMtNy43LDE0LjEtMTAuNCwyMS44LTYuMWwxNjYuMiw5My41YzcuNyw0LjMsMTAuNCwxNC4xLDYuMSwyMS44LTIuOSw1LjItOC40LDguMi0xNCw4LjJoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNMjU2LDE3OC41Yy04LjgsMC0xNi03LjItMTYtMTZ2LTkzLjVjMC04LjgsNy4yLTE2LDE2LTE2czE2LDcuMiwxNiwxNnY5My41YzAsOC44LTcuMiwxNi0xNiwxNloiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNODEuNywzNjMuM2MtNC45LTIuOS03LjktOC4xLTcuOS0xMy44di0xODdjMC02LDMuMy0xMS4yLDguMi0xMy45LDIuMy0xLjMsMjMuOC0xMy40LDIzLjgtMTMuNHYxODdsNTkuMy0zMy4zYzcuNy00LjMsMTcuNS0xLjYsMjEuOCw2LjEsNC4zLDcuNywxLjYsMTcuNS02LjEsMjEuOGwtOTAuOSw1MS4ycy01LjYtMy4xLTguMS00LjVoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNNDIyLjIsMzY3LjlsLTkwLjktNTEuMmMtNy43LTQuMy0xMC40LTE0LjEtNi4xLTIxLjhzMTQuMS0xMC40LDIxLjgtNi4xbDU5LjMsMzMuM3YtMTg3czIxLjUsMTIuMSwyMy45LDEzLjRjLjguNSwxLjYsMSwyLjMsMS42LDMuNiwyLjksNS44LDcuNCw1LjgsMTIuNHYxODdjMCw1LjctMywxMC45LTcuOSwxMy44LTIuNSwxLjUtOC4xLDQuNS04LjEsNC41aDBaIiBmaWxsPSJjdXJyZW50Q29sb3IiIC8+CiAgPHBhdGggZD0iTTMzOS4xLDIyNS4yYy0yLjcsMC01LjQtLjctNy44LTIuMWwtNzUuMy00Mi4zLTc1LjMsNDIuM2MtNy43LDQuMy0xNy41LDEuNi0yMS44LTYuMS00LjMtNy43LTEuNi0xNy41LDYuMS0yMS44bDgzLjEtNDYuOGM0LjktMi43LDEwLjgtMi43LDE1LjcsMGw4My4xLDQ2LjhjNy43LDQuMywxMC40LDE0LjEsNi4xLDIxLjgtMi45LDUuMi04LjQsOC4yLTE0LDguMmgwWiIgZmlsbD0iY3VycmVudENvbG9yIiAvPgogIDxwYXRoIGQ9Ik0yNTYsMzY1LjVjLTUuNiwwLTExLTIuOS0xNC04LjItNC4zLTcuNy0xLjYtMTcuNSw2LjEtMjEuOGw3NS00Mi4ydi04NC4xYzAtOC44LDcuMi0xNiwxNi0xNnMxNiw3LjIsMTYsMTZ2OTMuNWMwLDUuOC0zLjEsMTEuMS04LjIsMTRsLTgzLjEsNDYuOGMtMi41LDEuNC01LjIsMi4xLTcuOCwyLjFoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNMjU2LDM2NS41Yy0yLjcsMC01LjQtLjctNy44LTJsLTgzLjEtNDYuOGMtNS0yLjgtOC4yLTguMi04LjItMTR2LTkzLjVjMC04LjgsNy4yLTE2LDE2LTE2czE2LDcuMiwxNiwxNnY4NC4xbDUxLjEsMjguOHYtNjYuMWMwLTguOCw3LjItMTYsMTYtMTZzMTYsNy4yLDE2LDE2djEwMi45cy0zLDEuNi03LjksNC41Yy0yLjUsMS41LTUuMywyLjItOC4xLDIuMmgwWiIgZmlsbD0iY3VycmVudENvbG9yIiAvPgogIDxwYXRoIGQ9Ik0yNTYsMjcyYy01LjYsMC0xMS0yLjktMTQtOC4yLTQuMy03LjctMS42LTE3LjUsNi4xLTIxLjhsOTEtNTEuMiw3LjksNC40YzIuMSwxLjEsNC4yLDIuOCw2LjEsNi4xLDQuMyw3LjcsMS42LDE3LjUtNi4xLDIxLjhsLTgzLjEsNDYuOGMtMi41LDEuNC01LjIsMi4xLTcuOCwyLjFoMFoiIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KICA8cGF0aCBkPSJNMjU2LDIzNy42bC03NS4zLTQyLjNjLTcuNy00LjMtMTcuNS0xLjYtMjEuOCw2LjEtMS40LDIuNS0yLjEsNS4yLTIuMSw3Ljh2OS40bDkxLjMsNTEuM2MyLjUsMS40LDUuMiwyLjEsNy44LDIuMWgwdi0zNC40aDBaIiBmaWxsPSJjdXJyZW50Q29sb3IiIC8+Cjwvc3ZnPg=="

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
