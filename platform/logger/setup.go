package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

var Log zerolog.Logger

type LoggerConfig struct {
	Level  string
	Pretty bool
}

func Setup(cfg LoggerConfig) {
	var output io.Writer = os.Stdout
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	level := zerolog.DebugLevel
	if cfg.Level == "production" {
		level = zerolog.InfoLevel
	}

	Log = zerolog.New(output).Level(level).With().Timestamp().Logger()
}
