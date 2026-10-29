package dices

import (
	"math/rand"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"
)

type DicesRepository struct {
	logger     logger.Logger
	randomizer *rand.Rand
}

func NewDicesRepository(randomizer *rand.Rand, logger logger.Logger) *DicesRepository {
	if randomizer == nil {
		randomizer = rand.New(rand.NewSource(time.Now().UnixMilli()))
	}
	return &DicesRepository{logger: logger, randomizer: randomizer}
}
