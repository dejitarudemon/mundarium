package events

func (r *EventsRepository) info(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Info(msg, args...)
	}
}

func (r *EventsRepository) error(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Error(msg, args...)
	}
}

func (r *EventsRepository) debug(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Debug(msg, args...)
	}
}
