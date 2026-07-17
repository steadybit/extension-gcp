// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package e2e

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"

	"github.com/rs/zerolog/log"
)

// mockGcpServer is an in-process HTTP server that mimics just enough of the Compute Engine and Cloud SQL Admin
// REST APIs to exercise the extension-gcp discovery and state-change actions during e2e tests. All REST-based
// GCP clients in the extension share a single endpoint override (STEADYBIT_EXTENSION_COMPUTE_ENDPOINT), so one
// server serves every resource, dispatched by request path.
type mockGcpServer struct {
	*httptest.Server
	mu           sync.Mutex
	stopRequests []instanceRef
}

type instanceRef struct {
	Project  string
	Zone     string
	Instance string
}

// route maps an HTTP method + path pattern to a handler. Keeping pattern, method and handler in one table
// entry means adding a REST resource is a single addition rather than an edit in two disjoint places.
type route struct {
	method  string
	pattern *regexp.Regexp
	handle  func(m *mockGcpServer, w http.ResponseWriter, match []string)
}

var routes = []route{
	{http.MethodGet, regexp.MustCompile(`^/compute/v1/projects/([^/]+)/aggregated/instances$`),
		func(_ *mockGcpServer, w http.ResponseWriter, match []string) {
			writeJSON(w, instancesAggregatedList(match[1]))
		}},
	{http.MethodPost, regexp.MustCompile(`^/compute/v1/projects/([^/]+)/zones/([^/]+)/instances/([^/]+)/stop$`),
		func(m *mockGcpServer, w http.ResponseWriter, match []string) {
			m.recordStop(match[1], match[2], match[3])
			writeJSON(w, computeOperation("stop", match[2]))
		}},
	{http.MethodGet, regexp.MustCompile(`^/compute/v1/projects/([^/]+)/aggregated/routers$`),
		func(_ *mockGcpServer, w http.ResponseWriter, match []string) {
			writeJSON(w, routersAggregatedList(match[1]))
		}},
	{http.MethodGet, regexp.MustCompile(`^/compute/v1/projects/([^/]+)/aggregated/instanceGroupManagers$`),
		func(_ *mockGcpServer, w http.ResponseWriter, match []string) {
			writeJSON(w, migsAggregatedList(match[1]))
		}},
	{http.MethodGet, regexp.MustCompile(`^/compute/v1/projects/([^/]+)/aggregated/disks$`),
		func(_ *mockGcpServer, w http.ResponseWriter, match []string) {
			writeJSON(w, disksAggregatedList(match[1]))
		}},
	{http.MethodGet, regexp.MustCompile(`^/v1/projects/([^/]+)/instances$`),
		func(_ *mockGcpServer, w http.ResponseWriter, _ []string) { writeJSON(w, sqlInstancesList()) }},
}

func startMockGcpServer() *mockGcpServer {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic(fmt.Sprintf("mock gcp server: failed to listen: %v", err))
	}
	m := &mockGcpServer{}
	m.Server = &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: http.HandlerFunc(m.handle)},
	}
	m.Server.Start()
	log.Info().Str("url", m.Server.URL).Msg("Started mock GCP server")
	return m
}

func (m *mockGcpServer) handle(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Str("method", r.Method).Msg("Mock GCP request received")

	for _, rt := range routes {
		if r.Method != rt.method {
			continue
		}
		if match := rt.pattern.FindStringSubmatch(r.URL.Path); match != nil {
			rt.handle(m, w, match)
			return
		}
	}

	http.NotFound(w, r)
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	_, _ = w.Write([]byte(body))
}

func instancesAggregatedList(project string) string {
	return fmt.Sprintf(`{
  "kind": "compute#instanceAggregatedList",
  "id": "projects/%s/aggregated/instances",
  "items": {
    "zones/us-central1-a": {
      "instances": [
        {
          "kind": "compute#instance",
          "id": "42",
          "creationTimestamp": "2026-01-01T00:00:00.000-00:00",
          "name": "test",
          "description": "mock instance",
          "machineType": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a/machineTypes/e2-medium",
          "status": "RUNNING",
          "zone": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a",
          "cpuPlatform": "Intel Broadwell",
          "hostname": "test",
          "labels": {"team": "platform"}
        }
      ]
    },
    "zones/us-central1-b": {
      "warning": {
        "code": "NO_RESULTS_ON_PAGE",
        "message": "There are no results for scope 'zones/us-central1-b' on this page."
      }
    }
  },
  "selfLink": "https://www.googleapis.com/compute/v1/projects/%s/aggregated/instances"
}`, project, project, project, project)
}

func routersAggregatedList(project string) string {
	return fmt.Sprintf(`{
  "kind": "compute#routerAggregatedList",
  "id": "projects/%s/aggregated/routers",
  "items": {
    "regions/us-central1": {
      "routers": [
        {
          "kind": "compute#router",
          "id": "10",
          "name": "mock-router",
          "network": "https://www.googleapis.com/compute/v1/projects/%s/global/networks/default",
          "region": "https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1",
          "selfLink": "https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1/routers/mock-router",
          "nats": [
            {
              "name": "mock-nat",
              "sourceSubnetworkIpRangesToNat": "LIST_OF_SUBNETWORKS",
              "natIpAllocateOption": "AUTO_ONLY",
              "subnetworks": [
                {
                  "name": "https://www.googleapis.com/compute/v1/projects/%s/regions/us-central1/subnetworks/default",
                  "sourceIpRangesToNat": ["ALL_IP_RANGES"]
                }
              ]
            }
          ]
        }
      ]
    },
    "regions/europe-west1": {
      "warning": {
        "code": "NO_RESULTS_ON_PAGE",
        "message": "There are no results for scope 'regions/europe-west1' on this page."
      }
    }
  }
}`, project, project, project, project, project)
}

func migsAggregatedList(project string) string {
	return fmt.Sprintf(`{
  "kind": "compute#instanceGroupManagerAggregatedList",
  "id": "projects/%s/aggregated/instanceGroupManagers",
  "items": {
    "zones/us-central1-a": {
      "instanceGroupManagers": [
        {
          "kind": "compute#instanceGroupManager",
          "id": "20",
          "name": "mock-mig",
          "zone": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a",
          "selfLink": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a/instanceGroupManagers/mock-mig",
          "baseInstanceName": "mock",
          "instanceTemplate": "https://www.googleapis.com/compute/v1/projects/%s/global/instanceTemplates/mock-template",
          "targetSize": 3
        }
      ]
    },
    "zones/us-central1-b": {
      "warning": {
        "code": "NO_RESULTS_ON_PAGE",
        "message": "There are no results for scope 'zones/us-central1-b' on this page."
      }
    }
  }
}`, project, project, project, project)
}

func disksAggregatedList(project string) string {
	return fmt.Sprintf(`{
  "kind": "compute#diskAggregatedList",
  "id": "projects/%s/aggregated/disks",
  "items": {
    "zones/us-central1-a": {
      "disks": [
        {
          "kind": "compute#disk",
          "id": "30",
          "name": "mock-disk",
          "sizeGb": "100",
          "status": "READY",
          "type": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a/diskTypes/pd-ssd",
          "zone": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a",
          "selfLink": "https://www.googleapis.com/compute/v1/projects/%s/zones/us-central1-a/disks/mock-disk"
        }
      ]
    },
    "zones/us-central1-b": {
      "warning": {
        "code": "NO_RESULTS_ON_PAGE",
        "message": "There are no results for scope 'zones/us-central1-b' on this page."
      }
    }
  }
}`, project, project, project, project)
}

func sqlInstancesList() string {
	return `{
  "kind": "sql#instancesList",
  "items": [
    {
      "kind": "sql#instance",
      "name": "mock-sql",
      "databaseVersion": "POSTGRES_15",
      "region": "us-central1",
      "state": "RUNNABLE",
      "instanceType": "CLOUD_SQL_INSTANCE",
      "gceZone": "us-central1-a",
      "settings": {
        "tier": "db-custom-1-3840",
        "availabilityType": "REGIONAL",
        "dataDiskType": "PD_SSD",
        "dataDiskSizeGb": "20"
      }
    }
  ]
}`
}

func computeOperation(opType, zone string) string {
	return fmt.Sprintf(`{
  "kind": "compute#operation",
  "id": "1",
  "name": "operation-mock",
  "operationType": "%s",
  "status": "DONE",
  "progress": 100,
  "zone": "https://www.googleapis.com/compute/v1/zones/%s"
}`, opType, zone)
}

func (m *mockGcpServer) recordStop(project, zone, instance string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopRequests = append(m.stopRequests, instanceRef{Project: project, Zone: zone, Instance: instance})
}

func (m *mockGcpServer) StopRequests() []instanceRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]instanceRef(nil), m.stopRequests...)
}

// port returns the port the mock server is listening on. The host part of the
// server URL is bound to 0.0.0.0 and is not reachable from inside a minikube
// pod, so callers combine this port with `host.minikube.internal`.
func (m *mockGcpServer) port() string {
	u, err := url.Parse(m.Server.URL)
	if err != nil {
		panic(fmt.Sprintf("mock gcp server: cannot parse URL %q: %v", m.Server.URL, err))
	}
	return u.Port()
}
