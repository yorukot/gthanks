package domain

import "errors"

var (
	ErrInvalidTarget  = errors.New("invalid target")
	ErrTargetNotFound = errors.New("target not found")
	ErrRateLimited    = errors.New("github rate limited")
	ErrUpstream       = errors.New("github upstream error")
	ErrDatabase       = errors.New("database error")
)
