// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package e2e

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// mockComputeServer is an in-process HTTP server that mimics just enough of the
// Compute Engine REST API to exercise the extension-gcp discovery and state-change
// actions during e2e tests.
type mockComputeServer struct {
	*httptest.Server
	mu           sync.Mutex
	stopRequests []instanceRef
}

type instanceRef struct {
	Project  string
	Zone     string
	Instance string
}

var (
	aggregatedListPath = regexp.MustCompile(`^/compute/v1/projects/([^/]+)/aggregated/instances$`)
	stopInstancePath   = regexp.MustCompile(`^/compute/v1/projects/([^/]+)/zones/([^/]+)/instances/([^/]+)/stop$`)
)

func startMockComputeServer() *mockComputeServer {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic(fmt.Sprintf("mock compute server: failed to listen: %v", err))
	}
	m := &mockComputeServer{}
	m.Server = &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: http.HandlerFunc(m.handle)},
	}
	m.Server.Start()
	log.Info().Str("url", m.Server.URL).Msg("Started mock GCP Compute server")
	return m
}

func (m *mockComputeServer) handle(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Str("method", r.Method).Msg("Mock Compute request received")

	if match := aggregatedListPath.FindStringSubmatch(r.URL.Path); match != nil && r.Method == http.MethodGet {
		m.writeAggregatedList(w, match[1])
		return
	}
	if match := stopInstancePath.FindStringSubmatch(r.URL.Path); match != nil && r.Method == http.MethodPost {
		m.recordStop(match[1], match[2], match[3])
		writeOperation(w, "stop", match[2])
		return
	}

	http.NotFound(w, r)
}

func (m *mockComputeServer) writeAggregatedList(w http.ResponseWriter, project string) {
	body := fmt.Sprintf(`{
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
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	_, _ = w.Write([]byte(body))
}

func writeOperation(w http.ResponseWriter, opType, zone string) {
	body := fmt.Sprintf(`{
  "kind": "compute#operation",
  "id": "1",
  "name": "operation-mock",
  "operationType": "%s",
  "status": "DONE",
  "progress": 100,
  "zone": "https://www.googleapis.com/compute/v1/zones/%s"
}`, opType, zone)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	_, _ = w.Write([]byte(body))
}

func (m *mockComputeServer) recordStop(project, zone, instance string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopRequests = append(m.stopRequests, instanceRef{Project: project, Zone: zone, Instance: instance})
}

func (m *mockComputeServer) StopRequests() []instanceRef {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]instanceRef(nil), m.stopRequests...)
}

// hostPort extracts the "host:port" suffix of the mock server URL, suitable for
// constructing an endpoint reachable from inside minikube via host.minikube.internal.
func (m *mockComputeServer) hostPort() string {
	u := m.Server.URL
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	return u
}
