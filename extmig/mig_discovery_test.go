/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extmig

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/assert"
)

func TestParseScope(t *testing.T) {
	scope, loc := parseScope("zones/us-central1-a")
	assert.Equal(t, "zonal", scope)
	assert.Equal(t, "us-central1-a", loc)

	scope, loc = parseScope("regions/europe-west1")
	assert.Equal(t, "regional", scope)
	assert.Equal(t, "europe-west1", loc)

	scope, loc = parseScope("something-weird")
	assert.Equal(t, "unknown", scope)
	assert.Equal(t, "something-weird", loc)
}

func TestToMigTarget_Zonal(t *testing.T) {
	base := "base"
	tmpl := "projects/proj-a/global/instanceTemplates/tmpl-01"
	shape := "EVEN"
	upType := "PROACTIVE"
	repl := "SUBSTITUTE"
	minAct := "REPLACE"
	hc := "projects/proj-a/global/healthChecks/hc"
	zoneA := "projects/proj-a/zones/europe-west1-a"

	mig := &computepb.InstanceGroupManager{
		Name:             ptr("web-mig"),
		SelfLink:         ptr("https://compute.googleapis.com/.../instanceGroupManagers/web-mig"),
		TargetSize:       ptrI32(4),
		BaseInstanceName: &base,
		InstanceTemplate: &tmpl,
		DistributionPolicy: &computepb.DistributionPolicy{
			TargetShape: &shape,
			Zones:       []*computepb.DistributionPolicyZoneConfiguration{{Zone: &zoneA}, {}, {Zone: ptr("")}},
		},
		UpdatePolicy: &computepb.InstanceGroupManagerUpdatePolicy{
			Type:              &upType,
			ReplacementMethod: &repl,
			MinimalAction:     &minAct,
		},
		AutoHealingPolicies: []*computepb.InstanceGroupManagerAutoHealingPolicy{
			{HealthCheck: &hc},
			nil,
			{},
		},
		StatefulPolicy: &computepb.StatefulPolicy{},
	}

	target := toMigTarget(mig, "zonal", "europe-west1-a", "proj-a")

	assert.Equal(t, TargetIDMig, target.TargetType)
	assert.Equal(t, "web-mig", target.Label)
	assert.Equal(t, "https://compute.googleapis.com/.../instanceGroupManagers/web-mig", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"web-mig"}, target.Attributes["gcp.mig.name"])
	assert.Equal(t, []string{"zonal"}, target.Attributes[attrScope])
	assert.Equal(t, []string{"europe-west1-a"}, target.Attributes[attrLocation])
	assert.Equal(t, []string{"4"}, target.Attributes[attrTargetSize])
	assert.Equal(t, []string{"base"}, target.Attributes["gcp.mig.base-instance-name"])
	assert.Equal(t, []string{tmpl}, target.Attributes["gcp.mig.instance-template"])
	assert.Equal(t, []string{"EVEN"}, target.Attributes["gcp.mig.distribution-policy.target-shape"])
	// Nil / empty-name zones are filtered out.
	assert.Equal(t, []string{zoneA}, target.Attributes["gcp.mig.distribution-policy.zones"])
	assert.Equal(t, []string{"PROACTIVE"}, target.Attributes["gcp.mig.update-policy.type"])
	assert.Equal(t, []string{"SUBSTITUTE"}, target.Attributes["gcp.mig.update-policy.replacement-method"])
	assert.Equal(t, []string{"REPLACE"}, target.Attributes["gcp.mig.update-policy.minimal-action"])
	assert.Equal(t, []string{hc}, target.Attributes["gcp.mig.auto-healing-policies.health-check"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.mig.stateful-policy.configured"])
}

func TestToMigTarget_Sparse(t *testing.T) {
	mig := &computepb.InstanceGroupManager{
		Name:     ptr("bare"),
		SelfLink: ptr("selflink"),
	}
	target := toMigTarget(mig, "regional", "europe-west1", "proj-a")

	assert.Equal(t, "bare", target.Label)
	assert.Equal(t, []string{"regional"}, target.Attributes[attrScope])
	assert.Equal(t, []string{"europe-west1"}, target.Attributes[attrLocation])
	assert.Equal(t, []string{"0"}, target.Attributes[attrTargetSize])
	assert.NotContains(t, target.Attributes, "gcp.mig.distribution-policy.target-shape")
	assert.NotContains(t, target.Attributes, "gcp.mig.update-policy.type")
	assert.NotContains(t, target.Attributes, "gcp.mig.auto-healing-policies.health-check")
	// stateful-policy.configured is written unconditionally.
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.mig.stateful-policy.configured"])
}

func TestMigDescribeMethods(t *testing.T) {
	d := &migDiscovery{}
	assert.Equal(t, TargetIDMig, d.Describe().Id)
	assert.Equal(t, TargetIDMig, d.DescribeTarget().Id)
	assert.NotEmpty(t, d.DescribeAttributes())
}

func TestNewMigDiscovery(t *testing.T) {
	assert.NotNil(t, NewMigDiscovery())
}

func ptr(s string) *string { return &s }
func ptrI32(v int32) *int32 { return &v }
