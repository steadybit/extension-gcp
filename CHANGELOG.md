# Changelog

## v1.0.36

- chore(deps): bump cloud.google.com/go/pubsub/v2 from 2.6.1 to 2.6.2
- chore(deps): bump github.com/steadybit/action-kit/go/action_kit_test
- chore(deps): bump github.com/stretchr/testify from 1.11.1 to 1.12.1
- chore(deps): bump google.golang.org/grpc from 1.83.0 to 1.83.1

## v1.0.35

- feat(extvm): add gcp.region attribute to VM targets
- fix(extvm): copy VM labels and instance id to enriched targets

## v1.0.34

- chore(deps): bump steadybit kits and drop Go patch pin (#390)
- chore(deps): pin goreleaser build toolchain to go1.26.6
- chore(deps): use go-version-file, drop patch pin (go 1.26) (#389)

## v1.0.33

- chore(deps): bump cloud.google.com/go/compute from 1.65.0 to 1.66.0
- chore(deps): bump google.golang.org/api from 0.292.0 to 0.293.0
- chore(deps): bump google.golang.org/protobuf from 1.36.11 to 1.36.12

## v1.0.32

- chore(deps): bump google.golang.org/api from 0.291.0 to 0.292.0

## v1.0.31

- chore(deps): bump cloud.google.com/go/container from 1.51.0 to 1.53.0
- chore(deps): bump cloud.google.com/go/container from 1.53.0 to 1.53.1
- chore(deps): bump cloud.google.com/go/pubsub/v2 from 2.6.0 to 2.6.1
- chore(deps): bump cloud.google.com/go/redis from 1.23.0 to 1.24.0
- chore(deps): bump cloud.google.com/go/run from 1.21.0 to 1.22.0
- chore(deps): bump cloud.google.com/go/spanner from 1.91.0 to 1.93.0
- chore(deps): bump cloud.google.com/go/spanner from 1.93.0 to 1.94.0
- chore(deps): bump google.golang.org/api from 0.290.0 to 0.291.0
- chore(deps): bump google.golang.org/grpc from 1.82.0 to 1.82.1
- chore(deps): bump goreleaser/goreleaser from v2.17.0 to v2.17.1
- chore(deps): fix go mod tidy
- chore(deps): update dependencies
- cleanup: dedupe shared attribute descriptors, single-owner per attribute (#380)
- cleanup: extract duplicated attribute-name literals per Sonar go:S1192 (#361)
- cleanup: extract extvm + extnat Sonar leftovers (#364)
- feat: add discovery + attacks for 11 GCP services (GKE, MIG, Cloud NAT, Cloud SQL, Spanner, Pub/Sub, Memorystore, Cloud Run, Persistent Disk) (#334)
- feat: support filtering targets out of discovery
- feat: swap generic placeholder icons for official GCP product icons (#384)
- fix(attacks): tighten descriptions to one line and align Technology='GCP' (#366)
- fix(cloud-nat, mig, gke): three attack-implementation fixes (#381)
- fix(cloudsql): send required FailoverContext with settingsVersion (#383)
- fix: address correctness findings from PR #334 code review (#363)
- fix: shorten Pub/Sub topic persistence regions attribute name (#378)

## v1.0.30

- chore(deps): bump github.com/googleapis/gax-go/v2 from 2.22.0 to 2.23.0
- chore(deps): bump github.com/steadybit/action-kit/go/action_kit_sdk
- chore(deps): bump github.com/steadybit/discovery-kit/go/discovery_kit_sdk
- chore(deps): bump github.com/steadybit/extension-kit
- chore(deps): bump go to 1.26.5 (#357)
- chore(deps): bump google.golang.org/api from 0.285.0 to 0.286.0
- chore(deps): bump google.golang.org/api from 0.286.0 to 0.287.0
- chore(deps): bump google.golang.org/api from 0.287.1 to 0.288.0
- chore(deps): bump goreleaser/goreleaser from v2.16.0 to v2.17.0
- chore: add Claude Code workflows (#350)
- chore: silence SonarQube finding on secrets: inherit in Claude workflows
- refactor: register extension index via exthttp.RegisterRevisionedHandler (#358)

## v1.0.29

- chore(deps): bump github.com/steadybit/extension-kit
- chore(deps): bump google.golang.org/api from 0.284.0 to 0.285.0

## v1.0.28

- chore(deps): bump alpine from 3.23 to 3.24
- chore(deps): bump google.golang.org/api from 0.283.0 to 0.284.0

## v1.0.27

- chore(deps): bump cloud.google.com/go/compute from 1.63.0 to 1.64.0
- chore(deps): bump google.golang.org/api from 0.279.0 to 0.280.0
- chore(deps): bump google.golang.org/api from 0.280.0 to 0.282.0
- chore(deps): bump google.golang.org/api from 0.282.0 to 0.283.0
- chore(deps): bump goreleaser/goreleaser from v2.15.4 to v2.16.0
- chore: update to go 1.26.4
- feat: add weekly auto patch-release workflow

## v1.0.26

- Support discovery group attribute via `STEADYBIT_EXTENSION_DISCOVERY_GROUP` env var (or `discovery.group` Helm value) — when set, the extension adds `steadybit.group=<value>` to every discovered target
- Update dependencies

## v1.0.25

- Allow starting vm instances with the existing VM attack action.
- Bump Go to 1.26.3

## v1.0.24

- Support discovery across multiple GCP projects via `STEADYBIT_EXTENSION_PROJECT_IDS` (shared credentials) or `STEADYBIT_EXTENSION_PROJECTS_ADVANCED` (per-project service-account impersonation). The legacy `STEADYBIT_EXTENSION_PROJECT_ID` continues to work.

## v1.0.23

- Bump Go to 1.26.2
- Update dependencies

## v1.0.22

- Bump Go to 1.25.9
- Update dependencies

## v1.0.21

- Support if-none-match for the extension list endpoint
- Update dependencies

## v1.0.20

- feat(chart): split image.name into image.registry + image.name
- Support global.priorityClassName
- Support enrichment for argo rollouts
- Update alpine packages in Docker image to address CVEs
- Update dependencies

## v1.0.19

- Update dependencies

## v1.0.18

- Update dependencies

## v1.0.17

- Update dependencies

## v1.0.16

- Updated dependencies

## v1.0.15

- update dependencies

## v1.0.14

- update dependencies

## v1.0.13

- extend enrichment to more kubernetes types
- update dependencies
- Use uid instead of name for user statement in Dockerfile

## v1.0.12

- Set new `Technology` property in extension description
- Update dependencies (go 1.23)

## v1.0.11

- fix: don't declare two enrichment rules from vm to container

## v1.0.10

- Update dependencies

## v1.0.9

- Update dependencies (go 1.22)
- Refactored config object
- Refactored helm chart. Breaking changes. Please refer to the [README](README.md) for more information on how to authenticate.

## v1.0.8

- Update dependencies

## v1.0.7

- Update dependencies

## v1.0.6

- Update dependencies

## v1.0.5

- Update dependencies
- fixed enrichment for jvm instances

## v1.0.4

- Update dependencies
- add enrichment rules for kubernetes entities
- align attribute naming

## v1.0.3

- Update dependencies
- Added linux package
- refactored to use discovery-kit-sdk

## v1.0.2

- Added `pprof` endpoints for debugging purposes
- Update dependencies

## v1.0.1

- Possibility to exclude attributes from discovery

## v1.0.0

 - Initial release
