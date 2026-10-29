package dices

import (
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
)

// Roll evaluates the dice group and returns the throw result.
func (s *Service) Roll(throw repodices.DicesGroup) repodices.ThrowResult {
	result := s.repository.Roll(throw)
	s.info("dices are rolled", "type", "Service", "func", "Roll", "data", result)
	return result
}
