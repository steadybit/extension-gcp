/*
 * Copyright 2026 steadybit GmbH. All rights reserved.
 */

package utils

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-gcp/config"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

// GcpAccess represents the configured access to a single GCP project, including any pre-built client options
// required to authenticate (impersonation token sources or keyfile credentials).
type GcpAccess struct {
	ProjectID     string
	ClientOptions []option.ClientOption
}

var projects map[string]GcpAccess

// InitializeGcpAccess builds one GcpAccess per configured project. Must be called once after config.ValidateConfiguration.
// Projects whose client options fail to build are logged and skipped; the extension continues to operate with the
// remaining projects.
func InitializeGcpAccess(spec config.Specification) {
	projects = make(map[string]GcpAccess)
	for _, p := range config.ResolvedProjects() {
		opts, err := buildClientOptions(spec, p.ImpersonateServiceAccount)
		if err != nil {
			log.Error().Err(err).Str("project", p.ProjectID).Msg("Failed to build GCP client options; project will be ignored until extension is restarted.")
			continue
		}
		projects[p.ProjectID] = GcpAccess{ProjectID: p.ProjectID, ClientOptions: opts}
		if p.ImpersonateServiceAccount != "" {
			log.Info().Str("project", p.ProjectID).Str("impersonate", p.ImpersonateServiceAccount).Msg("Configured GCP project with service-account impersonation.")
		} else {
			log.Info().Str("project", p.ProjectID).Msg("Configured GCP project.")
		}
	}
	if len(projects) == 0 {
		log.Fatal().Msg("No usable GCP projects after client option initialization.")
	}
}

func buildClientOptions(spec config.Specification, impersonateServiceAccount string) ([]option.ClientOption, error) {
	if spec.ComputeEndpoint != "" {
		log.Warn().Str("endpoint", spec.ComputeEndpoint).Msg("STEADYBIT_EXTENSION_COMPUTE_ENDPOINT is set; GCP clients will skip authentication. This must only be used for testing.")
		return []option.ClientOption{option.WithEndpoint(spec.ComputeEndpoint), option.WithoutAuthentication()}, nil
	}

	var sourceOpts []option.ClientOption
	if spec.CredentialsKeyfilePath != "" {
		sourceOpts = append(sourceOpts, option.WithCredentialsFile(spec.CredentialsKeyfilePath))
	}
	if impersonateServiceAccount == "" {
		return sourceOpts, nil
	}
	ts, err := impersonate.CredentialsTokenSource(context.Background(), impersonate.CredentialsConfig{
		TargetPrincipal: impersonateServiceAccount,
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
	}, sourceOpts...)
	if err != nil {
		return nil, fmt.Errorf("create impersonation token source for '%s': %w", impersonateServiceAccount, err)
	}
	return []option.ClientOption{option.WithTokenSource(ts)}, nil
}

// GetGcpAccess returns the access entry for the given project ID, or an error if none is configured.
func GetGcpAccess(projectID string) (*GcpAccess, error) {
	a, ok := projects[projectID]
	if !ok {
		return nil, fmt.Errorf("no GCP access configured for project '%s'", projectID)
	}
	return &a, nil
}

// ForEveryConfiguredGcpAccess fans the supplier out across all configured projects using config.WorkerThreads goroutines.
// Errors from the supplier are logged per-project and do not abort the overall discovery.
func ForEveryConfiguredGcpAccess(
	supplier func(access *GcpAccess, ctx context.Context) ([]discovery_kit_api.Target, error),
	ctx context.Context,
	discovery string,
) ([]discovery_kit_api.Target, error) {
	count := len(projects)
	if count == 0 {
		return []discovery_kit_api.Target{}, nil
	}

	workers := config.Config.WorkerThreads
	if workers < 1 {
		workers = 1
	}
	if workers > count {
		workers = count
	}

	accessChan := make(chan GcpAccess, count)
	resultsChan := make(chan []discovery_kit_api.Target, count)

	for w := 1; w <= workers; w++ {
		go func(worker int) {
			for access := range accessChan {
				log.Trace().Str("project", access.ProjectID).Int("worker", worker).Msgf("Collecting %s", discovery)
				targets, err := supplier(&access, ctx)
				if err != nil {
					log.Err(err).Str("project", access.ProjectID).Msgf("Failed to collect %s", discovery)
				}
				resultsChan <- targets
			}
		}(w)
	}

	for _, a := range projects {
		accessChan <- a
	}
	close(accessChan)

	result := make([]discovery_kit_api.Target, 0)
	for i := 0; i < count; i++ {
		if targets := <-resultsChan; targets != nil {
			result = append(result, targets...)
		}
	}
	return result, nil
}

// SetProjectsForTest replaces the internal projects map. Intended for tests only.
func SetProjectsForTest(entries map[string]GcpAccess) {
	projects = entries
}
