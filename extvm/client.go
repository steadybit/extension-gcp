package extvm

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-gcp/utils"
)

func newInstancesClientForAccess(ctx context.Context, access *utils.GcpAccess) (*compute.InstancesClient, error) {
	client, err := compute.NewInstancesRESTClient(ctx, access.ClientOptions...)
	if err != nil {
		log.Error().Err(err).Str("project", access.ProjectID).Msg("Failed to create GCP instances client.")
		return nil, err
	}
	return client, nil
}
