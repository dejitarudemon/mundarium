package dices

func (t *RollDiceTool) info(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Info(msg, args...)
	}
}

func (t *RollDiceTool) warn(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Warn(msg, args...)
	}
}

func (t *RollDiceTool) error(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Error(msg, args...)
	}
}
