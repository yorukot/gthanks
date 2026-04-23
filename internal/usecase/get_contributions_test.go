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
	cache      *domain.CachedResponse
	imageCache *domain.CachedBinary
}

func (f *fakeStore) GetQueryCache(context.Context, string) (*domain.CachedResponse, error) {
	return f.cache, nil
}
func (f *fakeStore) SaveQueryCache(context.Context, domain.QueryCacheRecord) error { return nil }
func (f *fakeStore) GetImageCache(context.Context, string) (*domain.CachedBinary, error) {
	return f.imageCache, nil
}
func (f *fakeStore) SaveImageCache(context.Context, domain.ImageCacheRecord) error { return nil }
func (f *fakeStore) GetAvatarCache(context.Context, string) (*domain.CachedBinary, error) {
	return nil, nil
}
func (f *fakeStore) SaveAvatarCache(context.Context, domain.AvatarCacheRecord) error { return nil }
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

func TestBuildSummaryIncludesRepos(t *testing.T) {
	repos := []domain.Repo{
		{
			FullName: "yorukot/repo-a",
			HTMLURL:  "https://github.com/yorukot/repo-a",
			Contributors: []domain.Contributor{
				{
					IdentityKey:   "github_user:1",
					Login:         "alice",
					Contributions: 7,
				},
			},
		},
		{
			FullName: "yorukot/repo-b",
			HTMLURL:  "https://github.com/yorukot/repo-b",
			Contributors: []domain.Contributor{
				{
					IdentityKey:   "github_user:1",
					Login:         "alice",
					Contributions: 3,
				},
				{
					IdentityKey:   "github_user:2",
					Login:         "bob",
					Contributions: 5,
				},
			},
		},
	}

	summary := buildSummary(repos)
	if len(summary) != 2 {
		t.Fatalf("expected 2 summary items, got %d", len(summary))
	}

	if summary[0].Login != "alice" {
		t.Fatalf("expected alice first, got %q", summary[0].Login)
	}
	if summary[0].RepoCount != 2 {
		t.Fatalf("expected repo_count=2, got %d", summary[0].RepoCount)
	}
	if len(summary[0].Repos) != 2 {
		t.Fatalf("expected repo list in summary, got %#v", summary[0].Repos)
	}
	if summary[0].Repos[0].FullName != "yorukot/repo-a" || summary[0].Repos[0].Contributions != 7 {
		t.Fatalf("unexpected first repo summary: %#v", summary[0].Repos[0])
	}
	if summary[0].Repos[1].FullName != "yorukot/repo-b" || summary[0].Repos[1].Contributions != 3 {
		t.Fatalf("unexpected second repo summary: %#v", summary[0].Repos[1])
	}
}

func TestUserOrgModeExcludesForksByDefault(t *testing.T) {
	service := NewService(config.Config{
		CacheTTLSingleRepo: time.Hour,
		CacheTTLUserOrg:    3 * time.Hour,
	}, &fakeStore{}, &fakeGitHub{
		getRepoFn: func(context.Context, string, string) (domain.Repo, int, error) {
			return domain.Repo{}, 0, nil
		},
		listOwnerReposFn: func(context.Context, string) ([]domain.Repo, int, error) {
			return []domain.Repo{
				{FullName: "yorukot/repo-a", Owner: "yorukot", Name: "repo-a", Fork: false},
				{FullName: "yorukot/repo-b", Owner: "yorukot", Name: "repo-b", Fork: true},
			}, 1, nil
		},
		listContributorsFn: func(_ context.Context, _ string, repo string) ([]domain.Contributor, int, error) {
			return []domain.Contributor{{IdentityKey: "github_user:1", Login: repo, Contributions: 1}}, 1, nil
		},
	})

	resp, err := service.GetContributions(context.Background(), GetContributionsInput{
		Target:  "yorukot",
		Summary: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Repos) != 1 {
		t.Fatalf("expected only non-fork repos, got %d", len(resp.Repos))
	}
	if resp.Repos[0].FullName != "yorukot/repo-a" {
		t.Fatalf("unexpected repo included: %#v", resp.Repos)
	}
}

func TestUserOrgModeCanIncludeForks(t *testing.T) {
	service := NewService(config.Config{
		CacheTTLSingleRepo: time.Hour,
		CacheTTLUserOrg:    3 * time.Hour,
	}, &fakeStore{}, &fakeGitHub{
		getRepoFn: func(context.Context, string, string) (domain.Repo, int, error) {
			return domain.Repo{}, 0, nil
		},
		listOwnerReposFn: func(context.Context, string) ([]domain.Repo, int, error) {
			return []domain.Repo{
				{FullName: "yorukot/repo-a", Owner: "yorukot", Name: "repo-a", Fork: false},
				{FullName: "yorukot/repo-b", Owner: "yorukot", Name: "repo-b", Fork: true},
			}, 1, nil
		},
		listContributorsFn: func(_ context.Context, _ string, repo string) ([]domain.Contributor, int, error) {
			return []domain.Contributor{{IdentityKey: "github_user:1", Login: repo, Contributions: 1}}, 1, nil
		},
	})

	resp, err := service.GetContributions(context.Background(), GetContributionsInput{
		Target:       "yorukot",
		Summary:      true,
		IncludeForks: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Repos) != 2 {
		t.Fatalf("expected forks to be included, got %d repos", len(resp.Repos))
	}
}
