package domain

import "time"

type Metadata struct {
	Input            string    `json:"input"`
	NormalizedTarget string    `json:"normalized_target"`
	Mode             string    `json:"mode"`
	Status           string    `json:"status"`
	GeneratedAt      time.Time `json:"generated_at"`
}

type CacheInfo struct {
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Contributor struct {
	IdentityKey   string `json:"identity_key"`
	Login         string `json:"login,omitempty"`
	GitHubUserID  int64  `json:"github_user_id,omitempty"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	HTMLURL       string `json:"html_url,omitempty"`
	Type          string `json:"type,omitempty"`
	Contributions int    `json:"contributions"`
}

type Repo struct {
	FullName     string        `json:"full_name"`
	Owner        string        `json:"owner"`
	Name         string        `json:"name"`
	HTMLURL      string        `json:"html_url"`
	Private      bool          `json:"private"`
	Archived     bool          `json:"archived"`
	Fork         bool          `json:"fork"`
	FetchedAt    time.Time     `json:"fetched_at"`
	Contributors []Contributor `json:"contributors,omitempty"`
	Error        *ErrorDetail  `json:"error,omitempty"`
}

type SummaryItem struct {
	IdentityKey        string `json:"identity_key"`
	Login              string `json:"login,omitempty"`
	AvatarURL          string `json:"avatar_url,omitempty"`
	HTMLURL            string `json:"html_url,omitempty"`
	TotalContributions int    `json:"total_contributions"`
	RepoCount          int    `json:"repo_count"`
}

type ErrorDetail struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Details      string `json:"details,omitempty"`
	RepoFullName string `json:"repo_full_name,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
}

type ContributionResponse struct {
	Metadata           Metadata      `json:"metadata"`
	Cache              CacheInfo     `json:"cache"`
	Summary            []SummaryItem `json:"summary,omitempty"`
	Repos              []Repo        `json:"repos"`
	Errors             []ErrorDetail `json:"errors,omitempty"`
	GitHubRequestCount int           `json:"-"`
}

type CachedResponse struct {
	ResponseJSON []byte
	ExpiresAt    time.Time
}
