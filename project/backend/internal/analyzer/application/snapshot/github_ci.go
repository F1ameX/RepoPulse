package snapshot

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type CIReader interface {
	CollectCI(context.Context, domain.RepositoryMetadata, domain.Window) (domain.GitHubCI, []domain.SourceState)
}

type CICollector struct {
	reader CIReader
}

func NewCICollector(reader CIReader) CICollector {
	return CICollector{reader: reader}
}

func (c CICollector) Collect(ctx context.Context, a domain.Analysis, git worker.GitSession) (Collection, error) {
	ci, sources := c.reader.CollectCI(ctx, a.Repository, a.Window)
	ci.CollectConfigurations(git.Data())
	return Collection{GitHubCI: &ci, Sources: sources}, ctx.Err()
}
