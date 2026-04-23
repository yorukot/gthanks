package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gthanks/internal/config"
	"gthanks/internal/domain"
	"gthanks/internal/port"
)

type GetContributionsInput struct {
	Target       string
	Refresh      bool
	Summary      bool
	IncludeForks bool
}

type Service struct {
	cfg    config.Config
	store  port.Store
	github port.GitHubClient
}

func NewService(cfg config.Config, store port.Store, github port.GitHubClient) *Service {
	return &Service{cfg: cfg, store: store, github: github}
}

func (s *Service) Store() port.Store {
	return s.store
}

func (s *Service) GetImageCache(ctx context.Context, cacheKey string) (*domain.CachedBinary, error) {
	return s.store.GetImageCache(ctx, cacheKey)
}

func (s *Service) SaveImageCache(ctx context.Context, record domain.ImageCacheRecord) error {
	return s.store.SaveImageCache(ctx, record)
}

func (s *Service) GetContributions(ctx context.Context, input GetContributionsInput) (domain.ContributionResponse, error) {
	target, err := domain.ParseTarget(input.Target)
	if err != nil {
		return domain.ContributionResponse{}, err
	}

	cacheKey := buildCacheKey(target, input.Summary, input.IncludeForks)
	cached, err := s.store.GetQueryCache(ctx, cacheKey)
	if err != nil {
		return domain.ContributionResponse{}, fmt.Errorf("%w: get query cache: %v", domain.ErrDatabase, err)
	}

	now := time.Now().UTC()
	if !input.Refresh && cached != nil && cached.ExpiresAt.After(now) {
		var response domain.ContributionResponse
		if err := json.Unmarshal(cached.ResponseJSON, &response); err != nil {
			return domain.ContributionResponse{}, fmt.Errorf("%w: decode cached response: %v", domain.ErrDatabase, err)
		}
		response.Cache.Status = "hit"
		expiresAt := cached.ExpiresAt
		response.Cache.ExpiresAt = &expiresAt
		return response, nil
	}

	response, liveErr := s.fetchLive(ctx, target, input.Summary, input.IncludeForks)
	if liveErr == nil {
		response.Metadata.GeneratedAt = now
		if input.Refresh {
			response.Cache.Status = "refresh"
		} else {
			response.Cache.Status = "miss"
		}
		expiresAt := now.Add(ttlForMode(s.cfg, target.Mode))
		response.Cache.ExpiresAt = &expiresAt
		if err := s.persistResponse(ctx, cacheKey, target, response, expiresAt); err != nil {
			return domain.ContributionResponse{}, fmt.Errorf("%w: persist response: %v", domain.ErrDatabase, err)
		}
		return response, nil
	}

	if cached != nil {
		var stale domain.ContributionResponse
		if err := json.Unmarshal(cached.ResponseJSON, &stale); err == nil {
			stale.Cache.Status = "stale"
			expiresAt := cached.ExpiresAt
			stale.Cache.ExpiresAt = &expiresAt
			stale.Errors = append(stale.Errors, mapError(liveErr, "github refresh failed", "", 0))
			return stale, nil
		}
	}

	return domain.ContributionResponse{}, liveErr
}

func (s *Service) fetchLive(ctx context.Context, target domain.Target, includeSummary bool, includeForks bool) (domain.ContributionResponse, error) {
	response := domain.ContributionResponse{
		Metadata: domain.Metadata{
			Input:            target.Input,
			NormalizedTarget: target.NormalizedTarget,
			Mode:             target.Mode,
			Status:           "success",
		},
		Repos: []domain.Repo{},
	}

	switch target.Mode {
	case domain.ModeSingleRepo:
		repo, requests, err := s.github.GetRepository(ctx, target.Owner, target.Repo)
		response.GitHubRequestCount += requests
		if err != nil {
			return domain.ContributionResponse{}, err
		}

		contributors, requests, err := s.github.ListRepositoryContributors(ctx, target.Owner, target.Repo)
		response.GitHubRequestCount += requests
		if err != nil {
			return domain.ContributionResponse{}, err
		}
		repo.Contributors = contributors
		repo.FetchedAt = time.Now().UTC()
		response.Repos = append(response.Repos, repo)
	case domain.ModeUserOrOrg:
		repos, requests, err := s.github.ListOwnerRepositories(ctx, target.Owner)
		response.GitHubRequestCount += requests
		if err != nil {
			return domain.ContributionResponse{}, err
		}

		var partialErrors []domain.ErrorDetail
		for _, repo := range repos {
			if repo.Fork && !includeForks {
				continue
			}
			contributors, requests, err := s.github.ListRepositoryContributors(ctx, repo.Owner, repo.Name)
			response.GitHubRequestCount += requests
			if err != nil {
				detail := mapError(err, "failed to fetch repo contributors", repo.FullName, statusCode(err))
				repo.Error = &detail
				repo.FetchedAt = time.Now().UTC()
				response.Repos = append(response.Repos, repo)
				partialErrors = append(partialErrors, detail)
				continue
			}
			repo.Contributors = contributors
			repo.FetchedAt = time.Now().UTC()
			response.Repos = append(response.Repos, repo)
		}
		if len(partialErrors) > 0 {
			response.Metadata.Status = "partial_success"
			response.Errors = partialErrors
		}
	default:
		return domain.ContributionResponse{}, domain.ErrInvalidTarget
	}

	if includeSummary {
		response.Summary = buildSummary(response.Repos)
	}
	return response, nil
}

func (s *Service) persistResponse(ctx context.Context, cacheKey string, target domain.Target, response domain.ContributionResponse, expiresAt time.Time) error {
	for _, repo := range response.Repos {
		if repo.Error == nil {
			if err := s.store.SaveRepoSnapshot(ctx, target, repo); err != nil {
				return err
			}
		}
	}

	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return s.store.SaveQueryCache(ctx, domain.QueryCacheRecord{
		CacheKey:     cacheKey,
		Target:       target,
		ResponseJSON: raw,
		CacheStatus:  response.Cache.Status,
		ExpiresAt:    expiresAt,
	})
}

func buildSummary(repos []domain.Repo) []domain.SummaryItem {
	type aggregate struct {
		item  domain.SummaryItem
		repos map[string]domain.SummaryRepo
	}

	items := map[string]*aggregate{}
	for _, repo := range repos {
		for _, contributor := range repo.Contributors {
			entry, ok := items[contributor.IdentityKey]
			if !ok {
				entry = &aggregate{
					item: domain.SummaryItem{
						IdentityKey: contributor.IdentityKey,
						Login:       contributor.Login,
						AvatarURL:   contributor.AvatarURL,
						HTMLURL:     contributor.HTMLURL,
					},
					repos: map[string]domain.SummaryRepo{},
				}
				items[contributor.IdentityKey] = entry
			}
			entry.item.TotalContributions += contributor.Contributions
			entry.repos[repo.FullName] = domain.SummaryRepo{
				FullName:      repo.FullName,
				HTMLURL:       repo.HTMLURL,
				Contributions: contributor.Contributions,
			}
		}
	}

	summary := make([]domain.SummaryItem, 0, len(items))
	for _, entry := range items {
		entry.item.RepoCount = len(entry.repos)
		entry.item.Repos = make([]domain.SummaryRepo, 0, len(entry.repos))
		for _, repo := range entry.repos {
			entry.item.Repos = append(entry.item.Repos, repo)
		}
		sort.Slice(entry.item.Repos, func(i, j int) bool {
			if entry.item.Repos[i].Contributions != entry.item.Repos[j].Contributions {
				return entry.item.Repos[i].Contributions > entry.item.Repos[j].Contributions
			}
			return entry.item.Repos[i].FullName < entry.item.Repos[j].FullName
		})
		summary = append(summary, entry.item)
	}

	sort.Slice(summary, func(i, j int) bool {
		if summary[i].TotalContributions != summary[j].TotalContributions {
			return summary[i].TotalContributions > summary[j].TotalContributions
		}
		if summary[i].RepoCount != summary[j].RepoCount {
			return summary[i].RepoCount > summary[j].RepoCount
		}
		return summary[i].Login < summary[j].Login
	})

	return summary
}

func buildCacheKey(target domain.Target, summary bool, includeForks bool) string {
	return fmt.Sprintf("%s|summary=%t|include_forks=%t", target.NormalizedTarget, summary, includeForks)
}

func ttlForMode(cfg config.Config, mode string) time.Duration {
	if mode == domain.ModeSingleRepo {
		return cfg.CacheTTLSingleRepo
	}
	return cfg.CacheTTLUserOrg
}

func mapError(err error, message, repoFullName string, code int) domain.ErrorDetail {
	switch {
	case errors.Is(err, domain.ErrInvalidTarget):
		return domain.ErrorDetail{Code: "invalid_target", Message: message, RepoFullName: repoFullName, StatusCode: code}
	case errors.Is(err, domain.ErrTargetNotFound):
		return domain.ErrorDetail{Code: "not_found", Message: message, RepoFullName: repoFullName, StatusCode: code}
	case errors.Is(err, domain.ErrRateLimited):
		return domain.ErrorDetail{Code: "rate_limited", Message: message, RepoFullName: repoFullName, StatusCode: code}
	case errors.Is(err, domain.ErrUpstream):
		return domain.ErrorDetail{Code: "upstream_error", Message: message, RepoFullName: repoFullName, StatusCode: code}
	case errors.Is(err, context.DeadlineExceeded):
		return domain.ErrorDetail{Code: "timeout", Message: message, RepoFullName: repoFullName, StatusCode: code}
	default:
		return domain.ErrorDetail{Code: "internal_error", Message: message, RepoFullName: repoFullName, StatusCode: code}
	}
}

func statusCode(err error) int {
	type causer interface{ StatusCode() int }
	var sc causer
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	return 0
}
