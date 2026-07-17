/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extspanner

import (
	"testing"

	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"github.com/stretchr/testify/assert"
)

func TestToInstanceTarget_Populated(t *testing.T) {
	inst := &instancepb.Instance{
		Name:                      "projects/proj-a/instances/prod",
		DisplayName:               "Production",
		Config:                    "projects/proj-a/instanceConfigs/regional-europe-west1",
		Edition:                   instancepb.Instance_ENTERPRISE,
		InstanceType:              instancepb.Instance_PROVISIONED,
		State:                     instancepb.Instance_READY,
		NodeCount:                 3,
		ProcessingUnits:           3000,
		AutoscalingConfig:         &instancepb.AutoscalingConfig{},
		DefaultBackupScheduleType: instancepb.Instance_AUTOMATIC,
		Labels:                    map[string]string{"team": "core"},
	}

	target := toInstanceTarget(inst, "proj-a")

	assert.Equal(t, TargetIDInstance, target.TargetType)
	assert.Equal(t, "prod", target.Label)
	assert.Equal(t, "projects/proj-a/instances/prod", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"prod"}, target.Attributes["gcp.spanner.instance.name"])
	assert.Equal(t, []string{"Production"}, target.Attributes["gcp.spanner.instance.display-name"])
	assert.Equal(t, []string{"regional-europe-west1"}, target.Attributes[attrConfig])
	assert.Equal(t, []string{"ENTERPRISE"}, target.Attributes[attrEdition])
	assert.Equal(t, []string{"PROVISIONED"}, target.Attributes["gcp.spanner.instance.type"])
	assert.Equal(t, []string{"READY"}, target.Attributes["gcp.spanner.instance.state"])
	assert.Equal(t, []string{"3"}, target.Attributes["gcp.spanner.instance.node-count"])
	assert.Equal(t, []string{"3000"}, target.Attributes[attrProcessingUnits])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.spanner.instance.autoscaling.configured"])
	assert.Equal(t, []string{"AUTOMATIC"}, target.Attributes["gcp.spanner.instance.default-backup-schedule-type"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.spanner.instance.label.team"])
}

func TestToInstanceTarget_Sparse(t *testing.T) {
	inst := &instancepb.Instance{
		Name: "projects/proj-a/instances/sparse",
	}
	target := toInstanceTarget(inst, "proj-a")

	assert.Equal(t, "sparse", target.Label)
	assert.NotContains(t, target.Attributes, "gcp.spanner.instance.display-name")
	assert.NotContains(t, target.Attributes, attrConfig)
	assert.NotContains(t, target.Attributes, attrEdition)
	assert.NotContains(t, target.Attributes, "gcp.spanner.instance.state")
	assert.NotContains(t, target.Attributes, "gcp.spanner.instance.node-count")
	assert.NotContains(t, target.Attributes, attrProcessingUnits)
	// autoscaling.configured is written unconditionally; when AutoscalingConfig is nil, it's "false".
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.spanner.instance.autoscaling.configured"])
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
