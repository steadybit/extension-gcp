package extvm

import (
	compute "cloud.google.com/go/compute/apiv1"
	"context"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-gcp/config"
	"google.golang.org/api/option"
)

func GetGcpInstancesClient(ctx context.Context) (*compute.InstancesClient, error) {
	if config.Config.CredentialsKeyfilePath != "" {
		client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsFile(config.Config.CredentialsKeyfilePath))
		if err != nil {
			log.Error().Err(err).Msgf("Failed to create GCP client via file.")
			return nil, err
		}
		return client, nil
	}

	if config.Config.CredentialsKeyfileJson != "" {
		client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsJSON([]byte(config.Config.CredentialsKeyfileJson)))
		if err != nil {
			log.Error().Err(err).Msgf("Failed to create GCP client via json string.")
			return nil, err
		}
		return client, nil
	}

	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create GCP client.")
		return nil, err
	}
	return client, nil
}
