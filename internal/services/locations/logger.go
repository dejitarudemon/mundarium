package locations

func (s *Service) info(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Info(msg, args...)
	}
}

func (s *Service) warn(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Warn(msg, args...)
	}
}

func (s *Service) error(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Error(msg, args...)
	}
}

func (s *Service) debug(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(msg, args...)
	}
}
