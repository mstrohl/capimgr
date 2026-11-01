package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func resetGithubCache() {
	githubCacheMutex.Lock()
	githubCache = map[string]githubRepoInfo{}
	githubCacheMutex.Unlock()
}

func TestGithubRepoFromURL(t *testing.T) {
	repo, ok := githubRepoFromURL("https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/latest/infrastructure-components.yaml")
	if !ok || repo != "kubernetes-sigs/cluster-api-provider-aws" {
		t.Fatalf("repo inattendu: %q %v", repo, ok)
	}
	if _, ok := githubRepoFromURL("https://example.com/a/b"); ok {
		t.Fatal("un hôte non GitHub ne doit pas être reconnu")
	}
}

func TestFetchGithubRepoInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/active":
			w.Write([]byte(`{"archived":false}`))
		case "/repos/o/active/releases":
			w.Write([]byte(`[{"tag_name":"v2.0.0-rc.1","prerelease":true},{"tag_name":"v1.2.0"},{"tag_name":"nightly"},{"tag_name":"v1.0.0","draft":true}]`))
		case "/repos/o/old":
			w.Write([]byte(`{"archived":true}`))
		case "/repos/o/old/releases":
			w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	githubAPIBase = server.URL
	defer func() { githubAPIBase = "https://api.github.com" }()
	resetGithubCache()

	ctx := context.Background()
	if got := fetchGithubRepoStatus(ctx, "o/active"); got != providerStatusActive {
		t.Fatalf("dépôt actif attendu, obtenu %q", got)
	}
	versions, err := fetchGithubVersions(ctx, "o/active")
	if err != nil || len(versions) != 2 || versions[1].Version != "v1.2.0" || !versions[0].Prerelease {
		t.Fatalf("versions inattendues: %+v (%v)", versions, err)
	}
	if got := fetchGithubRepoStatus(ctx, "o/old"); got != providerStatusArchived {
		t.Fatalf("dépôt archivé attendu, obtenu %q", got)
	}
	if got := fetchGithubRepoStatus(ctx, "o/missing"); got != providerStatusArchived {
		t.Fatalf("un dépôt supprimé doit être archivé, obtenu %q", got)
	}
}

func TestGithubErrorsAreUnknownAndShortLived(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit", http.StatusForbidden)
	}))
	defer server.Close()
	githubAPIBase = server.URL
	defer func() { githubAPIBase = "https://api.github.com" }()
	resetGithubCache()

	if got := fetchGithubRepoStatus(context.Background(), "o/x"); got != providerStatusUnknown {
		t.Fatalf("statut inconnu attendu, obtenu %q", got)
	}
	if cached, ok := cachedGithubInfo("status:o/x"); !ok || time.Until(cached.expires) > githubErrorCacheTTL {
		t.Fatalf("cache court attendu: %+v", cached)
	}
	if _, err := fetchGithubVersions(context.Background(), "o/x"); err == nil {
		t.Fatal("une erreur est attendue quand GitHub refuse")
	}
}

func TestIsValidProviderVersion(t *testing.T) {
	for _, v := range []string{"latest", "v1.2.3", "1.2.3", "v1.2.3-rc.1"} {
		if !isValidProviderVersion(v) {
			t.Errorf("%q devrait être valide", v)
		}
	}
	for _, v := range []string{"", "v1", "1.2.3;rm", "--flag"} {
		if isValidProviderVersion(v) {
			t.Errorf("%q devrait être invalide", v)
		}
	}
}
