package events

import (
	"strings"

	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/google/uuid"
)

func requireEventID(id repoevents.ID) error {
	if uuid.UUID(id) == uuid.Nil {
		return toolerrs.NewInvalidRequest(toolerrs.ErrEventIDNil)
	}
	return nil
}

func parseEntityID(s string) (entities.ID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return entities.ID{}, toolerrs.NewInvalidRequest(toolerrs.ErrEntityIDEmpty)
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return entities.ID{}, toolerrs.NewInvalidRequest(toolerrs.ErrEntityIDEmpty)
	}
	return entities.ID(u), nil
}
