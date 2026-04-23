package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gthanks/internal/config"
	"gthanks/internal/domain"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type apiError struct {
	statusCode int
	message    string
}

func (e apiError) Error() string   { return e.message }
func (e apiError) StatusCode() int { return e.statusCode }

type repoDTO struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
	Archived bool   `json:"archived"`
	Fork     bool   `json:"fork"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type contributorDTO struct {
	ID            int64  `json:"id"`
	Login         string `json:"login"`
	AvatarURL     string `json:"avatar_url"`
	HTMLURL       string `json:"html_url"`
	Type          string `json:"type"`
	Contributions int    `json:"contributions"`
}

type errorDTO struct {
	Message string `json:"message"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.GitHubAPIBaseURL, "/"),
		token:   cfg.GitHubToken,
		httpClient: &http.Client{
			Timeout: cfg.GitHubTimeout,
		},
	}
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (domain.Repo, int, error) {
	var dto repoDTO
	count, err := c.getJSON(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo), &dto)
	if err != nil {
		return domain.Repo{}, count, err
	}
	return mapRepo(dto), count, nil
}

func (c *Client) ListOwnerRepositories(ctx context.Context, owner string) ([]domain.Repo, int, error) {
	repos, count, err := c.listRepos(ctx, fmt.Sprintf("/users/%s/repos?per_page=100", owner))
	if err == nil {
		return repos, count, nil
	}
	if !errors.Is(err, domain.ErrTargetNotFound) {
		return nil, count, err
	}

	orgRepos, orgCount, orgErr := c.listRepos(ctx, fmt.Sprintf("/orgs/%s/repos?per_page=100", owner))
	return orgRepos, count + orgCount, orgErr
}

func (c *Client) ListRepositoryContributors(ctx context.Context, owner, repo string) ([]domain.Contributor, int, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/contributors?per_page=100", owner, repo)
	var contributors []domain.Contributor
	requests := 0
	for endpoint != "" {
		var page []contributorDTO
		next, err := c.getPage(ctx, endpoint, &page)
		requests++
		if err != nil {
			return nil, requests, err
		}
		for _, item := range page {
			contributors = append(contributors, mapContributor(item))
		}
		endpoint = next
	}
	return contributors, requests, nil
}

func (c *Client) listRepos(ctx context.Context, endpoint string) ([]domain.Repo, int, error) {
	var repos []domain.Repo
	requests := 0
	for endpoint != "" {
		var page []repoDTO
		next, err := c.getPage(ctx, endpoint, &page)
		requests++
		if err != nil {
			return nil, requests, err
		}
		for _, item := range page {
			repos = append(repos, mapRepo(item))
		}
		endpoint = next
	}
	return repos, requests, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) (int, error) {
	_, err := c.doJSON(ctx, endpoint, out)
	return 1, err
}

func (c *Client) getPage(ctx context.Context, endpoint string, out any) (string, error) {
	return c.doJSON(ctx, endpoint, out)
}

func (c *Client) doJSON(ctx context.Context, endpoint string, out any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.resolveURL(endpoint), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var apiErr errorDTO
		_ = json.Unmarshal(body, &apiErr)
		message := strings.TrimSpace(apiErr.Message)
		if message == "" {
			message = strings.TrimSpace(string(body))
		}
		if message == "" {
			message = resp.Status
		}
		switch {
		case resp.StatusCode == http.StatusNotFound:
			return "", fmt.Errorf("%w: %s", domain.ErrTargetNotFound, apiError{statusCode: resp.StatusCode, message: message})
		case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
			return "", fmt.Errorf("%w: %s", domain.ErrRateLimited, apiError{statusCode: resp.StatusCode, message: message})
		case resp.StatusCode == http.StatusTooManyRequests:
			return "", fmt.Errorf("%w: %s", domain.ErrRateLimited, apiError{statusCode: resp.StatusCode, message: message})
		default:
			return "", fmt.Errorf("%w: %s", domain.ErrUpstream, apiError{statusCode: resp.StatusCode, message: message})
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return "", fmt.Errorf("%w: decode github response: %v", domain.ErrUpstream, err)
	}
	return parseNextLink(resp.Header.Get("Link")), nil
}

func (c *Client) resolveURL(endpoint string) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return c.baseURL + endpoint
}

func parseNextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start == -1 || end == -1 || start >= end {
			return ""
		}
		return part[start+1 : end]
	}
	return ""
}

func mapRepo(dto repoDTO) domain.Repo {
	return domain.Repo{
		FullName: dto.FullName,
		Owner:    dto.Owner.Login,
		Name:     dto.Name,
		HTMLURL:  dto.HTMLURL,
		Private:  dto.Private,
		Archived: dto.Archived,
		Fork:     dto.Fork,
	}
}

func mapContributor(dto contributorDTO) domain.Contributor {
	return domain.Contributor{
		IdentityKey:   identityKey(dto),
		Login:         dto.Login,
		GitHubUserID:  dto.ID,
		AvatarURL:     dto.AvatarURL,
		HTMLURL:       dto.HTMLURL,
		Type:          dto.Type,
		Contributions: dto.Contributions,
	}
}

func identityKey(dto contributorDTO) string {
	if dto.ID > 0 {
		return "github_user:" + strconv.FormatInt(dto.ID, 10)
	}
	if dto.Login != "" {
		return "github_login:" + strings.ToLower(dto.Login)
	}
	return "anonymous:" + strings.ToLower(strings.ReplaceAll(dto.HTMLURL, " ", "_"))
}

var _ = time.Second
var _ = url.URL{}
