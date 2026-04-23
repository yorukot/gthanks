package domain

import (
	"fmt"
	"strings"
)

const (
	ModeUserOrOrg  = "user_or_org"
	ModeSingleRepo = "single_repo"
)

type Target struct {
	Input            string `json:"input"`
	NormalizedTarget string `json:"normalized_target"`
	Mode             string `json:"mode"`
	Owner            string `json:"owner"`
	Repo             string `json:"repo,omitempty"`
}

func ParseTarget(input string) (Target, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Target{}, ErrInvalidTarget
	}
	if strings.Contains(trimmed, " ") {
		return Target{}, ErrInvalidTarget
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		if parts[0] == "" {
			return Target{}, ErrInvalidTarget
		}
		owner := strings.TrimSpace(parts[0])
		return Target{
			Input:            input,
			NormalizedTarget: owner,
			Mode:             ModeUserOrOrg,
			Owner:            owner,
		}, nil
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return Target{}, ErrInvalidTarget
		}
		owner := strings.TrimSpace(parts[0])
		repo := strings.TrimSpace(parts[1])
		return Target{
			Input:            input,
			NormalizedTarget: fmt.Sprintf("%s/%s", owner, repo),
			Mode:             ModeSingleRepo,
			Owner:            owner,
			Repo:             repo,
		}, nil
	default:
		return Target{}, ErrInvalidTarget
	}
}
