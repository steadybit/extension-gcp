/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudsql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/sqladmin/v1"
)

func TestToInstanceTarget_Populated(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name:             "primary",
		DatabaseVersion:  "POSTGRES_15",
		Region:           "europe-west1",
		GceZone:          "europe-west1-a",
		SecondaryGceZone: "europe-west1-b",
		State:            "RUNNABLE",
		InstanceType:     "CLOUD_SQL_INSTANCE",
		Settings: &sqladmin.Settings{
			Tier:             "db-custom-2-7680",
			AvailabilityType: "REGIONAL",
			BackupConfiguration: &sqladmin.BackupConfiguration{
				Enabled:                    true,
				PointInTimeRecoveryEnabled: true,
			},
			DeletionProtectionEnabled: true,
			IpConfiguration: &sqladmin.IpConfiguration{
				Ipv4Enabled:    false,
				PrivateNetwork: "projects/proj/global/networks/default",
				RequireSsl:     true,
			},
			DataDiskType:   "PD_SSD",
			DataDiskSizeGb: 100,
			MaintenanceWindow: &sqladmin.MaintenanceWindow{
				Day: 3,
			},
			UserLabels: map[string]string{"team": "core"},
		},
	}

	target := toInstanceTarget(inst, "proj-a")

	assert.Equal(t, TargetIDInstance, target.TargetType)
	assert.Equal(t, "primary", target.Label)
	assert.Equal(t, "projects/proj-a/instances/primary", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes["gcp.project.id"])
	assert.Equal(t, []string{"primary"}, target.Attributes["gcp.cloudsql.instance.name"])
	assert.Equal(t, []string{"POSTGRES_15"}, target.Attributes[attrDatabaseVersion])
	assert.Equal(t, []string{"europe-west1"}, target.Attributes[attrRegion])
	assert.Equal(t, []string{"europe-west1-a"}, target.Attributes["gcp.cloudsql.gce-zone"])
	assert.Equal(t, []string{"europe-west1-b"}, target.Attributes["gcp.cloudsql.secondary-gce-zone"])
	assert.Equal(t, []string{"RUNNABLE"}, target.Attributes["gcp.cloudsql.state"])
	assert.Equal(t, []string{"CLOUD_SQL_INSTANCE"}, target.Attributes["gcp.cloudsql.instance-type"])
	assert.Equal(t, []string{"db-custom-2-7680"}, target.Attributes[attrTier])
	assert.Equal(t, []string{"REGIONAL"}, target.Attributes[attrAvailabilityType])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudsql.backup-enabled"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudsql.point-in-time-recovery-enabled"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudsql.deletion-protection-enabled"])
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.cloudsql.public-network-access"])
	assert.Equal(t, []string{"projects/proj/global/networks/default"}, target.Attributes["gcp.cloudsql.private-network"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudsql.require-ssl"])
	assert.Equal(t, []string{"PD_SSD"}, target.Attributes["gcp.cloudsql.disk-type"])
	assert.Equal(t, []string{"100"}, target.Attributes["gcp.cloudsql.disk-size-gb"])
	assert.Equal(t, []string{"3"}, target.Attributes["gcp.cloudsql.maintenance-window.day"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.cloudsql.label.team"])
}

func TestToInstanceTarget_Sparse(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name: "sparse",
	}
	target := toInstanceTarget(inst, "proj-a")

	assert.Equal(t, "sparse", target.Label)
	assert.NotContains(t, target.Attributes, attrDatabaseVersion)
	assert.NotContains(t, target.Attributes, attrRegion)
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.gce-zone")
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.state")
	assert.NotContains(t, target.Attributes, attrTier)
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.deletion-protection-enabled")
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.disk-size-gb")
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.maintenance-window.day")
}

func TestToInstanceTarget_SettingsWithoutBackupOrIpConfig(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name: "primary",
		Settings: &sqladmin.Settings{
			Tier: "db-custom-2-7680",
		},
	}
	target := toInstanceTarget(inst, "proj-a")

	assert.Equal(t, []string{"db-custom-2-7680"}, target.Attributes[attrTier])
	// DeletionProtectionEnabled is always set from settings when settings present
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.cloudsql.deletion-protection-enabled"])
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.backup-enabled")
	assert.NotContains(t, target.Attributes, "gcp.cloudsql.public-network-access")
}

func TestDescribeMethods(t *testing.T) {
	d := &instanceDiscovery{}
	assert.Equal(t, TargetIDInstance, d.Describe().Id)
	assert.Equal(t, TargetIDInstance, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewInstanceDiscovery(t *testing.T) {
	d := NewInstanceDiscovery()
	assert.NotNil(t, d)
}
