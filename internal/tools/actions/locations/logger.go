package locations

import "github.com/dejitarudemon/mundarium/internal/libs/logger"

func info(log logger.Logger, msg string, args ...any) {
	if log != nil {
		log.Info(msg, args...)
	}
}

func errorf(log logger.Logger, msg string, args ...any) {
	if log != nil {
		log.Error(msg, args...)
	}
}

func debug(log logger.Logger, msg string, args ...any) {
	if log != nil {
		log.Debug(msg, args...)
	}
}
