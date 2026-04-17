/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProjects_LegacySingleProjectAccepted(t *testing.T) {
	spec := &Specification{ProjectID: "proj-a"}
	require.NoError(t, validateProjects(spec))
}

func TestValidateProjects_PluralListAccepted(t *testing.T) {
	spec := &Specification{ProjectIDs: []string{"proj-a", "proj-b"}}
	require.NoError(t, validateProjects(spec))
}

func TestValidateProjects_AdvancedAccepted(t *testing.T) {
	spec := &Specification{ProjectsAdvanced: ProjectsAdvanced{
		{ProjectID: "proj-a", ImpersonateServiceAccount: "sa@proj-a.iam.gserviceaccount.com"},
	}}
	require.NoError(t, validateProjects(spec))
}

func TestValidateProjects_NoSourceRejected(t *testing.T) {
	spec := &Specification{}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no GCP project configured")
}

func TestValidateProjects_LegacyAndPluralRejected(t *testing.T) {
	spec := &Specification{ProjectID: "proj-a", ProjectIDs: []string{"proj-b"}}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of")
}

func TestValidateProjects_PluralAndAdvancedRejected(t *testing.T) {
	spec := &Specification{
		ProjectIDs:       []string{"proj-a"},
		ProjectsAdvanced: ProjectsAdvanced{{ProjectID: "proj-b"}},
	}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of")
}

func TestValidateProjects_LegacyAndAdvancedRejected(t *testing.T) {
	spec := &Specification{
		ProjectID:        "proj-a",
		ProjectsAdvanced: ProjectsAdvanced{{ProjectID: "proj-b"}},
	}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one of")
}

func TestValidateProjects_PluralDuplicateProjectRejected(t *testing.T) {
	spec := &Specification{ProjectIDs: []string{"proj-a", "proj-a"}}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate projectId 'proj-a'")
}

func TestValidateProjects_AdvancedDuplicateProjectRejected(t *testing.T) {
	spec := &Specification{ProjectsAdvanced: ProjectsAdvanced{
		{ProjectID: "proj-a"},
		{ProjectID: "proj-a", ImpersonateServiceAccount: "sa@..."},
	}}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate projectId 'proj-a'")
}

func TestValidateProjects_AdvancedEmptyProjectIdRejected(t *testing.T) {
	spec := &Specification{ProjectsAdvanced: ProjectsAdvanced{
		{ProjectID: "", ImpersonateServiceAccount: "sa@..."},
	}}
	err := validateProjects(spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty projectId")
}

func TestProjectsAdvancedUnmarshalText_EmptyReturnsEmpty(t *testing.T) {
	var p ProjectsAdvanced
	require.NoError(t, p.UnmarshalText(nil))
	assert.Empty(t, p)
}

func TestProjectsAdvancedUnmarshalText_Json(t *testing.T) {
	var p ProjectsAdvanced
	require.NoError(t, p.UnmarshalText([]byte(`[{"projectId":"proj-a","impersonateServiceAccount":"sa@proj-a.iam.gserviceaccount.com"}]`)))
	require.Len(t, p, 1)
	assert.Equal(t, "proj-a", p[0].ProjectID)
	assert.Equal(t, "sa@proj-a.iam.gserviceaccount.com", p[0].ImpersonateServiceAccount)
}

func TestResolvedProjects_PrefersAdvanced(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	Config = Specification{
		ProjectIDs:       []string{"plural-a"},
		ProjectsAdvanced: ProjectsAdvanced{{ProjectID: "adv-a"}},
	}
	resolved := ResolvedProjects()
	require.Len(t, resolved, 1)
	assert.Equal(t, "adv-a", resolved[0].ProjectID)
}

func TestResolvedProjects_FallsBackToLegacy(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	Config = Specification{ProjectID: "legacy-a"}
	resolved := ResolvedProjects()
	require.Len(t, resolved, 1)
	assert.Equal(t, "legacy-a", resolved[0].ProjectID)
	assert.Empty(t, resolved[0].ImpersonateServiceAccount)
}

func TestResolvedProjects_UsesPlural(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	Config = Specification{ProjectIDs: []string{"plural-a", "plural-b"}}
	resolved := ResolvedProjects()
	require.Len(t, resolved, 2)
	assert.Equal(t, "plural-a", resolved[0].ProjectID)
	assert.Equal(t, "plural-b", resolved[1].ProjectID)
}
