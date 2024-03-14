package logging

import (
	"log/slog"
	"os"
)

func init() {
	logHandler := slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	slog.SetDefault(slog.New(logHandler))
}
