package source

import (
	"context"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

type ContestSource interface {
	Name() string
	Platform() domain.Platform
	FetchContests(ctx context.Context) ([]domain.Contest, error)
}
