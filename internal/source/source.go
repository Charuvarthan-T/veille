package source

import (
	"context"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

type ContestSource interface {
	Name() string
	Platform() domain.Platform
	FetchUpcoming(ctx context.Context) ([]domain.Contest, error)
}
