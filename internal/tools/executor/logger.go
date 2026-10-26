package executor

func (e *Executor) info(msg string, args ...any) {
	if e.logger != nil {
		e.logger.Info(msg, args...)
	}
}

func (e *Executor) error(msg string, args ...any) {
	if e.logger != nil {
		e.logger.Error(msg, args...)
	}
}

func (e *Executor) debug(msg string, args ...any) {
	if e.logger != nil {
		e.logger.Debug(msg, args...)
	}
}
