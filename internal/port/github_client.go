package port

import (
	"context"

	"gthanks/internal/domain"
)

type GitHubClient interface {
	GetRepository(ctx context.Context, owner, repo string) (domain.Repo, int, error)
	ListOwnerRepositories(ctx context.Context, owner string) ([]domain.Repo, int, error)
	ListRepositoryContributors(ctx context.Context, owner, repo string) ([]domain.Contributor, int, error)
}
