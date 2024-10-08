package service

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// Logger returns the default slog.Logger with additional information from context.Context.
func Logger(ctx context.Context) *slog.Logger {
	rid := RequestIDFromContext(ctx)
	uid := UserIDFromContext(ctx)

	logger := slog.Default()
	if rid != "" {
		logger = logger.With("request_id", rid)
	}
	if uid != uuid.Nil {
		logger = logger.With("user_id", uid)
	}
	return logger
}

func LogInfo(ctx context.Context, err error) {
	Logger(ctx).Info(err.Error())
}

func LogError(ctx context.Context, err error) {
	Logger(ctx).Error(err.Error())
}

func LogInternalError(ctx context.Context, err error) {
	if ErrorStatusCode(err) == http.StatusInternalServerError {
		LogError(ctx, err)
	}
}
