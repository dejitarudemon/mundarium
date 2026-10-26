package messages

func (r *MessagesRepository) info(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Info(msg, args...)
	}
}

func (r *MessagesRepository) error(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Error(msg, args...)
	}
}

func (r *MessagesRepository) debug(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Debug(msg, args...)
	}
}
