package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	providerStatusActive   = "active"
	providerStatusArchived = "archived"
	providerStatusUnknown  = "unknown"

	githubCacheTTL      = 30 * time.Minute
	githubErrorCacheTTL = time.Minute
	// Limite le nombre d'appels simultanés pour ne pas déclencher la protection anti-abus de GitHub
	githubMaxConcurrency = 4
)

// ProviderVersion décrit une release publiée par le dépôt d'un provider.
type ProviderVersion struct {
	Version     string    `json:"version"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"publishedAt"`
}

// ProviderVersionsResponse est la réponse de GET /api/providers/{id}/versions.
type ProviderVersionsResponse struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Default  string            `json:"default,omitempty"`
	Versions []ProviderVersion `json:"versions"`
}

type githubRepoInfo struct {
	Status   string
	Versions []ProviderVersion
	expires  time.Time
}

var (
	githubAPIBase = "https://api.github.com"
	githubClient  = &http.Client{Timeout: 8 * time.Second}

	githubCache      = map[string]githubRepoInfo{}
	githubCacheMutex sync.Mutex

	versionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.\-+]+)?$`)
)

// githubRepoFromURL extrait owner/repo d'une URL de dépôt clusterctl.
func githubRepoFromURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host != "github.com" {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func githubGet(ctx context.Context, path string, target interface{}) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBase+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "capimgr")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := githubClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return resp.StatusCode, fmt.Errorf("GitHub a répondu %d pour %s: %s", resp.StatusCode, path, strings.TrimSpace(string(body)))
	}
	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(target)
}

func cachedGithubInfo(key string) (githubRepoInfo, bool) {
	githubCacheMutex.Lock()
	defer githubCacheMutex.Unlock()
	cached, ok := githubCache[key]
	return cached, ok && time.Now().Before(cached.expires)
}

func storeGithubInfo(key string, info githubRepoInfo) {
	githubCacheMutex.Lock()
	githubCache[key] = info
	githubCacheMutex.Unlock()
}

// fetchGithubRepoStatus retourne le statut (actif/archivé/inconnu) d'un dépôt avec un seul appel API.
// Un dépôt introuvable (404) est considéré comme archivé : il n'existe plus.
func fetchGithubRepoStatus(ctx context.Context, repo string) string {
	key := "status:" + repo
	if cached, ok := cachedGithubInfo(key); ok {
		return cached.Status
	}
	info := githubRepoInfo{Status: providerStatusUnknown, expires: time.Now().Add(githubErrorCacheTTL)}
	var meta struct {
		Archived bool `json:"archived"`
	}
	code, err := githubGet(ctx, "/repos/"+repo, &meta)
	switch {
	case code == http.StatusNotFound:
		info.Status = providerStatusArchived
		info.expires = time.Now().Add(githubCacheTTL)
	case err == nil:
		info.Status = providerStatusActive
		if meta.Archived {
			info.Status = providerStatusArchived
		}
		info.expires = time.Now().Add(githubCacheTTL)
	}
	storeGithubInfo(key, info)
	return info.Status
}

// fetchGithubVersions liste les releases d'un dépôt (cache 30 min). Les erreurs ne sont pas mises en cache longtemps.
func fetchGithubVersions(ctx context.Context, repo string) ([]ProviderVersion, error) {
	key := "versions:" + repo
	if cached, ok := cachedGithubInfo(key); ok {
		return cached.Versions, nil
	}
	var releases []struct {
		TagName     string    `json:"tag_name"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
	}
	if _, err := githubGet(ctx, "/repos/"+repo+"/releases?per_page=50", &releases); err != nil {
		return nil, err
	}
	versions := []ProviderVersion{}
	for _, r := range releases {
		if r.Draft || !versionPattern.MatchString(r.TagName) {
			continue
		}
		versions = append(versions, ProviderVersion{Version: r.TagName, Prerelease: r.Prerelease, PublishedAt: r.PublishedAt})
	}
	storeGithubInfo(key, githubRepoInfo{Versions: versions, expires: time.Now().Add(githubCacheTTL)})
	return versions, nil
}

// providerStatuses résout en parallèle le statut de chaque dépôt (ID provider -> statut).
func providerStatuses(ctx context.Context, repos map[string]string) map[string]string {
	statuses := make(map[string]string, len(repos))
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, githubMaxConcurrency)
	for id, repo := range repos {
		wg.Add(1)
		go func(id, repo string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			status := fetchGithubRepoStatus(ctx, repo)
			mu.Lock()
			statuses[id] = status
			mu.Unlock()
		}(id, repo)
	}
	wg.Wait()
	return statuses
}

func isValidProviderVersion(version string) bool {
	return version == "latest" || versionPattern.MatchString(version)
}

// getProviderVersions gère GET /api/providers/{id}/versions.
func getProviderVersions(w http.ResponseWriter, r *http.Request, id string) {
	repoData, err := getClusterctlRepositories()
	if err != nil {
		http.Error(w, "Erreur lors de la lecture de la configuration clusterctl", http.StatusInternalServerError)
		return
	}
	data, ok := repoData[id]
	if !ok {
		http.Error(w, "Provider inconnu de clusterctl", http.StatusNotFound)
		return
	}

	response := ProviderVersionsResponse{ID: id, Status: providerStatusUnknown, Versions: []ProviderVersion{}}
	if repo, ok := githubRepoFromURL(data["url"]); ok {
		response.Status = fetchGithubRepoStatus(r.Context(), repo)
		versions, err := fetchGithubVersions(r.Context(), repo)
		if err != nil {
			log.Printf("Erreur versions %s: %v", id, err)
			http.Error(w, "Impossible de lister les versions depuis GitHub (limite de requêtes ? définir GITHUB_TOKEN)", http.StatusBadGateway)
			return
		}
		response.Versions = versions
		for _, v := range versions {
			if !v.Prerelease {
				response.Default = v.Version
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
