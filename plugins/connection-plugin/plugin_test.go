package connection

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	cloudapi "github.com/simplecloudapp/cloud-api/go"
)

func TestInitialSyncRegistersAllAvailableServers(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("state") != "AVAILABLE" || request.URL.Query().Get("type") != "SERVER" {
			t.Errorf("unexpected filters: %s", request.URL.RawQuery)
		}
		body := `{
			"count": 2,
			"servers": [
				{"server_id":"one","ip":"127.0.0.1","port":25565,"numerical_id":1,"state":"AVAILABLE","server_group":{"name":"Lobby","type":"SERVER"}},
				{"server_id":"two","ip":"127.0.0.2","port":25566,"numerical_id":2,"state":"AVAILABLE","server_group":{"name":"Lobby","type":"SERVER"}}
			]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}

	client, err := cloudapi.NewClient(cloudapi.Options{
		ControllerURL: "http://controller.invalid",
		NetworkID:     "network",
		NetworkSecret: "secret",
		HTTPClient:    httpClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := newFakeRegistry()
	registration := defaultConnectionConfig().Registration
	app := &application{
		ctx:       context.Background(),
		client:    client,
		registrar: newRegistrar(registry, func() RegistrationConfig { return registration }, logr.Discard()),
		log:       logr.Discard(),
	}

	if err := app.syncInitialServersOnce(); err != nil {
		t.Fatal(err)
	}
	if registry.Server("Lobby-1") == nil || registry.Server("Lobby-2") == nil {
		t.Fatal("available servers were not registered")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
