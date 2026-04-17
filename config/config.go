/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	//STEADYBIT_EXTENSION_CREDENTIALS_KEYFILE_PATH
	CredentialsKeyfilePath string `json:"credentialsKeyfilePath" required:"false" split_words:"true"`
	//STEADYBIT_EXTENSION_PROJECT_ID - legacy single-project config, kept for backward compatibility.
	ProjectID string `json:"projectId" required:"false" split_words:"true"`
	//STEADYBIT_EXTENSION_PROJECT_IDS - comma-separated list of GCP project IDs to discover. Uses the same credentials for all projects.
	ProjectIDs []string `json:"projectIds" required:"false" split_words:"true"`
	//STEADYBIT_EXTENSION_PROJECTS_ADVANCED - JSON array of {projectId, impersonateServiceAccount}. Enables per-project service-account impersonation.
	ProjectsAdvanced ProjectsAdvanced `json:"projectsAdvanced" required:"false" split_words:"true"`
	//STEADYBIT_EXTENSION_WORKER_THREADS - number of goroutines used to fan out discovery across projects.
	WorkerThreads int `json:"workerThreads" required:"false" split_words:"true" default:"1"`
	//STEADYBIT_EXTENSION_COMPUTE_ENDPOINT - override the Compute API endpoint. Intended for testing only; when set the client skips authentication.
	ComputeEndpoint               string   `json:"computeEndpoint" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesVM []string `json:"discoveryAttributesExcludesVM" required:"false" split_words:"true"`
	EnrichVMDataForTargetTypes    []string `json:"EnrichVMDataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_kubernetes.argo-rollout,com.steadybit.extension_kubernetes.kubernetes-deployment,com.steadybit.extension_kubernetes.kubernetes-pod,com.steadybit.extension_kubernetes.kubernetes-daemonset,com.steadybit.extension_kubernetes.kubernetes-statefulset,com.steadybit.extension_http.client-location,com.steadybit.extension_jmeter.location,com.steadybit.extension_k6.location,com.steadybit.extension_gatling.location"`
}

type ProjectAdvanced struct {
	ProjectID                 string `json:"projectId"`
	ImpersonateServiceAccount string `json:"impersonateServiceAccount"`
}

type ProjectsAdvanced []ProjectAdvanced

func (p *ProjectsAdvanced) UnmarshalText(text []byte) error {
	if len(text) == 0 || string(text) == "[]" {
		*p = ProjectsAdvanced{}
		return nil
	}
	return json.Unmarshal(text, (*[]ProjectAdvanced)(p))
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}

	Config.ProjectID = strings.TrimSpace(Config.ProjectID)
	Config.ProjectIDs = trimAndFilter(Config.ProjectIDs)
}

func ValidateConfiguration() {
	if err := validateProjects(&Config); err != nil {
		log.Fatal().Err(err).Msg("Invalid GCP project configuration.")
	}
	log.Info().Msgf("Configured %d GCP project(s) for discovery.", len(ResolvedProjects()))
}

// ResolvedProjects returns the effective list of projects derived from ProjectID, ProjectIDs and ProjectsAdvanced.
// It assumes ValidateConfiguration has been called and the mutual-exclusion rules have been enforced.
func ResolvedProjects() []ProjectAdvanced {
	if len(Config.ProjectsAdvanced) > 0 {
		return Config.ProjectsAdvanced
	}
	ids := Config.ProjectIDs
	if len(ids) == 0 && Config.ProjectID != "" {
		ids = []string{Config.ProjectID}
	}
	projects := make([]ProjectAdvanced, 0, len(ids))
	for _, id := range ids {
		projects = append(projects, ProjectAdvanced{ProjectID: id})
	}
	return projects
}

func validateProjects(c *Specification) error {
	sources := 0
	if c.ProjectID != "" {
		sources++
	}
	if len(c.ProjectIDs) > 0 {
		sources++
	}
	if len(c.ProjectsAdvanced) > 0 {
		sources++
	}
	if sources == 0 {
		return fmt.Errorf("no GCP project configured: set STEADYBIT_EXTENSION_PROJECT_IDS or STEADYBIT_EXTENSION_PROJECTS_ADVANCED")
	}
	if sources > 1 {
		return fmt.Errorf("only one of STEADYBIT_EXTENSION_PROJECT_ID, STEADYBIT_EXTENSION_PROJECT_IDS, STEADYBIT_EXTENSION_PROJECTS_ADVANCED may be set")
	}
	if err := checkDuplicateIDs("STEADYBIT_EXTENSION_PROJECT_IDS", c.ProjectIDs); err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, p := range c.ProjectsAdvanced {
		if strings.TrimSpace(p.ProjectID) == "" {
			return fmt.Errorf("STEADYBIT_EXTENSION_PROJECTS_ADVANCED: every entry must have a non-empty projectId")
		}
		if _, dup := seen[p.ProjectID]; dup {
			return fmt.Errorf("STEADYBIT_EXTENSION_PROJECTS_ADVANCED: duplicate projectId '%s'", p.ProjectID)
		}
		seen[p.ProjectID] = struct{}{}
	}
	return nil
}

func checkDuplicateIDs(source string, ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%s: duplicate projectId '%s'", source, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func trimAndFilter(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
