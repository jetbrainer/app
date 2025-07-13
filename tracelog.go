package app

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog"
)

type Logger struct {
	l               *zerolog.Logger
	invalidLevelKey string
}

type OptionF func(*Logger)

func WithInvalidLevelKey(key string) OptionF {
	return func(l *Logger) {
		l.invalidLevelKey = key
	}
}

func NewLogger(l *zerolog.Logger, options ...OptionF) *Logger {
	logger := &Logger{
		l:               l,
		invalidLevelKey: "INVALID_PGX_LOG_LEVEL",
	}

	for _, option := range options {
		option(logger)
	}

	return logger
}

func (l *Logger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	var event *zerolog.Event

	switch level {
	case tracelog.LogLevelTrace:
		event = l.l.Debug().Str("PGX_LOG_LEVEL", level.String())
	case tracelog.LogLevelDebug:
		event = l.l.Debug()
	case tracelog.LogLevelInfo:
		event = l.l.Info()
	case tracelog.LogLevelWarn:
		event = l.l.Warn()
	case tracelog.LogLevelError:
		event = l.l.Error()
	default:
		event = l.l.Error().Str(l.invalidLevelKey, fmt.Sprintf("invalid pgx log level: %v", level))
	}

	for k, v := range data {
		event = event.Interface(k, v)
	}

	event.Ctx(ctx).Msg(msg)
}
