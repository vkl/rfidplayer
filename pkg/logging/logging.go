package logging

import (
	"log/slog"
	"os"
)

func init() {
	logHandler := slog.NewTextHandler(
		os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn, AddSource: true})
	slog.SetDefault(slog.New(logHandler))
}
