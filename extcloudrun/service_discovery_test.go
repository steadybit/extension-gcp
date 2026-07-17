/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package extcloudrun

import (
	"testing"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestParseServiceName(t *testing.T) {
	loc, name := parseServiceName("projects/my-proj/locations/europe-west1/services/my-svc")
	assert.Equal(t, "europe-west1", loc)
	assert.Equal(t, "my-svc", name)

	loc, name = parseServiceName("not-a-valid-name")
	assert.Equal(t, "", loc)
	assert.Equal(t, "", name)
}

func TestToServiceTarget_Populated(t *testing.T) {
	svc := &runpb.Service{
		Name:               "projects/proj-a/locations/europe-west1/services/svc-a",
		Ingress:            runpb.IngressTraffic_INGRESS_TRAFFIC_ALL,
		LaunchStage:        1, // LAUNCH_STAGE_UNSPECIFIED is 0; use 1 to avoid the "skip if zero" guard
		InvokerIamDisabled: true,
		IapEnabled:         true,
		Scaling: &runpb.ServiceScaling{
			MinInstanceCount: 2,
			ScalingMode:      runpb.ServiceScaling_AUTOMATIC,
		},
		Template: &runpb.RevisionTemplate{
			MaxInstanceRequestConcurrency: 42,
			Timeout:                       durationpb.New(60 * time.Second),
			ServiceAccount:                "sa@proj-a.iam.gserviceaccount.com",
			Scaling: &runpb.RevisionScaling{
				MinInstanceCount: 1,
				MaxInstanceCount: 5,
			},
		},
		Urls: []string{"https://svc-a-xyz.a.run.app"},
		Labels: map[string]string{
			"team": "core",
		},
	}
	target := toServiceTarget(svc, "proj-a")

	assert.Equal(t, TargetIDService, target.TargetType)
	assert.Equal(t, "svc-a", target.Label)
	assert.Equal(t, "projects/proj-a/locations/europe-west1/services/svc-a", target.Id)
	assert.Equal(t, []string{"proj-a"}, target.Attributes[attrProjectID])
	assert.Equal(t, []string{"svc-a"}, target.Attributes["gcp.cloudrun.service.name"])
	assert.Equal(t, []string{"europe-west1"}, target.Attributes[attrLocation])
	assert.Equal(t, []string{"INGRESS_TRAFFIC_ALL"}, target.Attributes[attrIngress])
	assert.Contains(t, target.Attributes, "gcp.cloudrun.service.launch-stage")
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudrun.service.invoker-iam-disabled"])
	assert.Equal(t, []string{"true"}, target.Attributes["gcp.cloudrun.service.iap-enabled"])
	assert.Equal(t, []string{"2"}, target.Attributes["gcp.cloudrun.service.scaling.min-instance-count"])
	assert.Equal(t, []string{"AUTOMATIC"}, target.Attributes["gcp.cloudrun.service.scaling.scaling-mode"])
	assert.Equal(t, []string{"42"}, target.Attributes["gcp.cloudrun.service.template.max-instance-request-concurrency"])
	assert.Equal(t, []string{"1m0s"}, target.Attributes["gcp.cloudrun.service.template.timeout"])
	assert.Equal(t, []string{"sa@proj-a.iam.gserviceaccount.com"}, target.Attributes["gcp.cloudrun.service.template.service-account"])
	assert.Equal(t, []string{"1"}, target.Attributes["gcp.cloudrun.service.template.scaling.min-instance-count"])
	assert.Equal(t, []string{"5"}, target.Attributes["gcp.cloudrun.service.template.scaling.max-instance-count"])
	assert.Equal(t, []string{"https://svc-a-xyz.a.run.app"}, target.Attributes["gcp.cloudrun.service.urls"])
	assert.Equal(t, []string{"core"}, target.Attributes["gcp.cloudrun.service.label.team"])
}

func TestToServiceTarget_Sparse(t *testing.T) {
	svc := &runpb.Service{
		Name: "projects/proj-a/locations/europe-west1/services/svc-b",
	}
	target := toServiceTarget(svc, "proj-a")

	assert.Equal(t, "svc-b", target.Label)
	// Ingress/LaunchStage unset -> not present
	assert.NotContains(t, target.Attributes, attrIngress)
	assert.NotContains(t, target.Attributes, "gcp.cloudrun.service.launch-stage")
	// Scaling nil -> min-instance-count absent
	assert.NotContains(t, target.Attributes, "gcp.cloudrun.service.scaling.min-instance-count")
	// Template nil -> no template attrs
	assert.NotContains(t, target.Attributes, "gcp.cloudrun.service.template.timeout")
	assert.NotContains(t, target.Attributes, "gcp.cloudrun.service.urls")
	// InvokerIamDisabled/IapEnabled are always emitted (default false)
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.cloudrun.service.invoker-iam-disabled"])
	assert.Equal(t, []string{"false"}, target.Attributes["gcp.cloudrun.service.iap-enabled"])
}

func TestDescribeMethods(t *testing.T) {
	d := &serviceDiscovery{}
	desc := d.Describe()
	assert.Equal(t, TargetIDService, desc.Id)
	target := d.DescribeTarget()
	assert.Equal(t, TargetIDService, target.Id)
	attrs := d.DescribeAttributes()
	assert.NotEmpty(t, attrs)
}

func TestNewServiceDiscovery(t *testing.T) {
	d := NewServiceDiscovery()
	assert.NotNil(t, d)
}
