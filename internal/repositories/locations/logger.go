package locations

func (r *LocationRepository) info(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Info(msg, args...)
	}
}

func (r *LocationRepository) error(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Error(msg, args...)
	}
}

func (r *LocationRepository) debug(msg string, args ...any) {
	if r.logger != nil {
		r.logger.Debug(msg, args...)
	}
}
