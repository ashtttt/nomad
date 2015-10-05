package fingerprint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestGCEFingerprint_nonGCE(t *testing.T) {
	f := NewEnvGCEFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatalf("Should be false without test server")
	}
}

func testFingerprint_GCE(t *testing.T, withExternalIp bool) {
	f := NewEnvGCEFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(GCE_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in GCE ENV test: %s", err)
	}
	if withExternalIp {
		routes.Endpoints = append(routes.Endpoints, &endpoint{
			Uri:         "/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip",
			ContentType: "text/plain",
			Body:        "104.44.55.66",
		})
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, ok := r.Header["Metadata-Flavor"]
		if !ok {
			t.Fatal("Metadata-Flavor not present in HTTP request header")
		}
		if value[0] != "Google" {
			t.Fatalf("Expected Metadata-Flavor Google, saw %s", value[0])
		}

		found := false
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
			found = true
		}

		if !found {
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()
	os.Setenv("GCE_ENV_URL", ts.URL+"/computeMetadata/v1/instance/")

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !ok {
		t.Fatalf("should apply")
	}

	keys := []string{
		"platform.gce.id",
		"platform.gce.hostname",
		"platform.gce.zone",
		"platform.gce.machine-type",
		"platform.gce.zone",
		"platform.gce.tag.abc",
		"platform.gce.tag.def",
		"platform.gce.attr.ghi",
		"platform.gce.attr.jkl",
		"network.ip-address",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, node, k)
	}

	if len(node.Links) == 0 {
		t.Fatalf("Empty links for Node in GCE Fingerprint test")
	}

	// Make sure Links contains the GCE ID.
	for _, k := range []string{"gce"} {
		assertNodeLinksContains(t, node, k)
	}

	assertNodeAttributeEquals(t, node, "platform.gce.id", "12345")
	assertNodeAttributeEquals(t, node, "platform.gce.hostname", "instance-1.c.project.internal")
	assertNodeAttributeEquals(t, node, "platform.gce.zone", "us-central1-f")
	assertNodeAttributeEquals(t, node, "platform.gce.machine-type", "n1-standard-1")

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP != "10.240.0.5" {
		t.Fatalf("Expected Network Resource to have IP 10.240.0.5, saw %s", net.IP)
	}
	if net.CIDR != "10.240.0.5/32" {
		t.Fatalf("Expected Network Resource to have CIDR 10.240.0.5/32, saw %s", net.CIDR)
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}

	assertNodeAttributeEquals(t, node, "network.ip-address", "10.240.0.5")
	if withExternalIp {
		assertNodeAttributeEquals(t, node, "platform.gce.external-ip", "104.44.55.66")
	} else if _, ok := node.Attributes["platform.gce.external-ip"]; ok {
		t.Fatal("platform.gce.external-ip is set without an external IP")
	}

	assertNodeAttributeEquals(t, node, "platform.gce.tag.abc", "true")
	assertNodeAttributeEquals(t, node, "platform.gce.tag.def", "true")
	assertNodeAttributeEquals(t, node, "platform.gce.attr.ghi", "111")
	assertNodeAttributeEquals(t, node, "platform.gce.attr.jkl", "222")
}

const GCE_routes = `
{
  "endpoints": [
    {
      "uri": "/computeMetadata/v1/instance/id",
      "content-type": "text/plain",
      "body": "12345"
    },
    {
      "uri": "/computeMetadata/v1/instance/hostname",
      "content-type": "text/plain",
      "body": "instance-1.c.project.internal"
    },
    {
      "uri": "/computeMetadata/v1/instance/zone",
      "content-type": "text/plain",
      "body": "projects/555555/zones/us-central1-f"
    },
    {
      "uri": "/computeMetadata/v1/instance/machine-type",
      "content-type": "text/plain",
      "body": "projects/555555/machineTypes/n1-standard-1"
    },
    {
      "uri": "/computeMetadata/v1/instance/network-interfaces/0/ip",
      "content-type": "text/plain",
      "body": "10.240.0.5"
    },
    {
      "uri": "/computeMetadata/v1/instance/tags",
      "content-type": "application/json",
      "body": "[\"abc\", \"def\"]"
    },
    {
      "uri": "/computeMetadata/v1/instance/attributes/?recursive=true",
      "content-type": "application/json",
      "body": "{\"ghi\":\"111\",\"jkl\":\"222\"}"
    }
  ]
}
`

func TestFingerprint_GCEWithExternalIp(t *testing.T) {
	testFingerprint_GCE(t, true)
}

func TestFingerprint_GCEWithoutExternalIp(t *testing.T) {
	testFingerprint_GCE(t, false)
}
