package shared

import (
	"log/slog"
	"os"
)


func NewLogger(l slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
