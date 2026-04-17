/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package utils

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-gcp/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGcpAccess_NotFound(t *testing.T) {
	t.Cleanup(func() { SetProjectsForTest(nil) })
	SetProjectsForTest(map[string]GcpAccess{})

	_, err := GetGcpAccess("missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestGetGcpAccess_Found(t *testing.T) {
	t.Cleanup(func() { SetProjectsForTest(nil) })
	SetProjectsForTest(map[string]GcpAccess{
		"proj-a": {ProjectID: "proj-a"},
	})

	got, err := GetGcpAccess("proj-a")
	require.NoError(t, err)
	assert.Equal(t, "proj-a", got.ProjectID)
}

func TestForEveryConfiguredGcpAccess_NoProjectsReturnsEmpty(t *testing.T) {
	t.Cleanup(func() { SetProjectsForTest(nil) })
	SetProjectsForTest(map[string]GcpAccess{})

	targets, err := ForEveryConfiguredGcpAccess(func(*GcpAccess, context.Context) ([]discovery_kit_api.Target, error) {
		t.Fatal("supplier should not be called when no projects are configured")
		return nil, nil
	}, context.Background(), "test")

	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestForEveryConfiguredGcpAccess_AggregatesAcrossProjects(t *testing.T) {
	t.Cleanup(func() {
		SetProjectsForTest(nil)
		config.Config.WorkerThreads = 0
	})
	SetProjectsForTest(map[string]GcpAccess{
		"proj-a": {ProjectID: "proj-a"},
		"proj-b": {ProjectID: "proj-b"},
	})
	config.Config.WorkerThreads = 2

	targets, err := ForEveryConfiguredGcpAccess(func(access *GcpAccess, _ context.Context) ([]discovery_kit_api.Target, error) {
		return []discovery_kit_api.Target{
			{Id: "vm-" + access.ProjectID, TargetType: "test", Attributes: map[string][]string{"gcp.project.id": {access.ProjectID}}},
		}, nil
	}, context.Background(), "test")

	require.NoError(t, err)
	require.Len(t, targets, 2)
	sort.Slice(targets, func(i, j int) bool { return targets[i].Id < targets[j].Id })
	assert.Equal(t, "vm-proj-a", targets[0].Id)
	assert.Equal(t, "vm-proj-b", targets[1].Id)
}

func TestForEveryConfiguredGcpAccess_IsolatesErrorsPerProject(t *testing.T) {
	t.Cleanup(func() {
		SetProjectsForTest(nil)
		config.Config.WorkerThreads = 0
	})
	SetProjectsForTest(map[string]GcpAccess{
		"proj-a": {ProjectID: "proj-a"},
		"proj-b": {ProjectID: "proj-b"},
	})
	config.Config.WorkerThreads = 1

	targets, err := ForEveryConfiguredGcpAccess(func(access *GcpAccess, _ context.Context) ([]discovery_kit_api.Target, error) {
		if access.ProjectID == "proj-a" {
			return nil, errors.New("simulated project-a failure")
		}
		return []discovery_kit_api.Target{{Id: "vm-" + access.ProjectID, TargetType: "test"}}, nil
	}, context.Background(), "test")

	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "vm-proj-b", targets[0].Id)
}
