package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gthanks/internal/config"
	"gthanks/internal/domain"
)

type fakeStore struct {
	cache *domain.CachedResponse
}

func (f *fakeStore) GetQueryCache(context.Context, string) (*domain.CachedResponse, error) {
	return f.cache, nil
}
func (f *fakeStore) SaveQueryCache(context.Context, domain.QueryCacheRecord) error { return nil }
func (f *fakeStore) SaveRepoSnapshot(context.Context, domain.Target, domain.Repo) error {
	return nil
}

type fakeGitHub struct {
	getRepoFn          func(context.Context, string, string) (domain.Repo, int, error)
	listOwnerReposFn   func(context.Context, string) ([]domain.Repo, int, error)
	listContributorsFn func(context.Context, string, string) ([]domain.Contributor, int, error)
}

func (f *fakeGitHub) GetRepository(ctx context.Context, owner, repo string) (domain.Repo, int, error) {
	return f.getRepoFn(ctx, owner, repo)
}
func (f *fakeGitHub) ListOwnerRepositories(ctx context.Context, owner string) ([]domain.Repo, int, error) {
	return f.listOwnerReposFn(ctx, owner)
}
func (f *fakeGitHub) ListRepositoryContributors(ctx context.Context, owner, repo string) ([]domain.Contributor, int, error) {
	return f.listContributorsFn(ctx, owner, repo)
}

func TestGetContributionsCacheHit(t *testing.T) {
	now := time.Now().UTC()
	cachedResponse := domain.ContributionResponse{
		Metadata: domain.Metadata{
			Input:            "yorukot",
			NormalizedTarget: "yorukot",
			Mode:             domain.ModeUserOrOrg,
			Status:           "success",
			GeneratedAt:      now,
		},
		Cache: domain.CacheInfo{Status: "miss"},
		Repos: []domain.Repo{{FullName: "yorukot/repo", Owner: "yorukot", Name: "repo"}},
	}
	raw, err := json.Marshal(cachedResponse)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(config.Config{
		CacheTTLSingleRepo: time.Hour,
		CacheTTLUserOrg:    3 * time.Hour,
	}, &fakeStore{
		cache: &domain.CachedResponse{
			ResponseJSON: raw,
			ExpiresAt:    now.Add(time.Hour),
		},
	}, &fakeGitHub{
		getRepoFn: func(context.Context, string, string) (domain.Repo, int, error) {
			return domain.Repo{}, 0, errors.New("should not call github")
		},
		listOwnerReposFn: func(context.Context, string) ([]domain.Repo, int, error) {
			return nil, 0, errors.New("should not call github")
		},
		listContributorsFn: func(context.Context, string, string) ([]domain.Contributor, int, error) {
			return nil, 0, errors.New("should not call github")
		},
	})

	resp, err := service.GetContributions(context.Background(), GetContributionsInput{Target: "yorukot", Summary: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cache.Status != "hit" {
		t.Fatalf("expected cache hit, got %q", resp.Cache.Status)
	}
	if len(resp.Repos) != 1 {
		t.Fatalf("expected cached repos, got %d", len(resp.Repos))
	}
}

func TestGetContributionsStaleFallback(t *testing.T) {
	now := time.Now().UTC()
	cachedResponse := domain.ContributionResponse{
		Metadata: domain.Metadata{
			Input:            "yorukot/superfile",
			NormalizedTarget: "yorukot/superfile",
			Mode:             domain.ModeSingleRepo,
			Status:           "success",
			GeneratedAt:      now.Add(-2 * time.Hour),
		},
		Repos: []domain.Repo{{FullName: "yorukot/superfile", Owner: "yorukot", Name: "superfile"}},
	}
	raw, err := json.Marshal(cachedResponse)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(config.Config{
		CacheTTLSingleRepo: time.Hour,
		CacheTTLUserOrg:    3 * time.Hour,
	}, &fakeStore{
		cache: &domain.CachedResponse{
			ResponseJSON: raw,
			ExpiresAt:    now.Add(-time.Minute),
		},
	}, &fakeGitHub{
		getRepoFn: func(context.Context, string, string) (domain.Repo, int, error) {
			return domain.Repo{}, 1, domain.ErrRateLimited
		},
		listOwnerReposFn: func(context.Context, string) ([]domain.Repo, int, error) { return nil, 0, nil },
		listContributorsFn: func(context.Context, string, string) ([]domain.Contributor, int, error) {
			return nil, 0, nil
		},
	})

	resp, err := service.GetContributions(context.Background(), GetContributionsInput{Target: "yorukot/superfile", Summary: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cache.Status != "stale" {
		t.Fatalf("expected stale cache, got %q", resp.Cache.Status)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Code != "rate_limited" {
		t.Fatalf("expected stale fallback error detail, got %#v", resp.Errors)
	}
}
