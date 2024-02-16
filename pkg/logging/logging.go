package logging

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func init() {
	logHandler := slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	Log = slog.New(logHandler)
}
