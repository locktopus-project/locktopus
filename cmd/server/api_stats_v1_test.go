package main_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/locktopus-project/locktopus/internal/constants"
	locktopusclient "github.com/locktopus-project/locktopus/pkg/client/v1"
)

const statsNamespaceName = "stats_namespace"

func TestStats_BeforeInitiatingNamespace(t *testing.T) {
	url := fmt.Sprintf("http://%s/stats_v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, statsNamespaceName)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("cannot query Locktopus server: %s", err)
		return
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Status code is not 404")
		return
	}
}

func TestStats_AfterInitiatingNamespace(t *testing.T) {
	url := fmt.Sprintf("http://%s/stats_v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, statsNamespaceName)

	_, err := locktopusclient.MakeClient(locktopusclient.ConnectionOptions{
		Url: fmt.Sprintf("ws://%s/v1?%s=%s", serverAddress, constants.NamespaceQueryParameterName, statsNamespaceName),
	})
	if err != nil {
		t.Fatalf("cannot connect to Locktopus server: %s", err)
		return
	}

	// make http get request to url
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("cannot query Locktopus server: %s", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status code is not 200")
		return
	}

	defer resp.Body.Close()

	// check for successfully parsing json
	err = json.NewDecoder(resp.Body).Decode(&map[string]interface{}{})
	if err != nil {
		t.Fatalf("cannot parse response body: %s", err)
		return
	}
}
