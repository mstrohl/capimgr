package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestOpenAPIRoutes(t *testing.T) {
	mux := http.NewServeMux()
	registerOpenAPIRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	for _, path := range []string{
		"/api/docs/",
		"/api/docs/swagger-ui.css",
		"/api/docs/swagger-ui-bundle.js",
		"/api/docs/swagger-ui-standalone-preset.js",
		"/api/docs/index.css",
	} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s: got status %d", path, response.StatusCode)
			}
		})
	}

	response, err := http.Get(server.URL + "/api/docs/swagger-initializer.js")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `url: "../openapi.yaml"`) {
		t.Fatalf("Swagger initializer does not point to the local definition: status %d", response.StatusCode)
	}

	response, err = http.Get(server.URL + "/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err = io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	if err := yaml.Unmarshal(body, &spec); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(spec.Servers) != 1 || spec.Servers[0].URL != "/api" {
		t.Fatalf("unexpected API definition or server: status %d, servers %+v", response.StatusCode, spec.Servers)
	}
}
