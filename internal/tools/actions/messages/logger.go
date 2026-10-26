package messages

func (t *CreateMessageTool) info(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Info(msg, args...)
	}
}

func (t *CreateMessageTool) warn(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Warn(msg, args...)
	}
}

func (t *CreateMessageTool) error(msg string, args ...any) {
	if t.logger != nil {
		t.logger.Error(msg, args...)
	}
}
