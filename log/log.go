// Package log controls how logging
package log

import (
	"context"
	"log/slog"
)

var logger *slog.Logger

// SetLogger sets the logger that will be used as the destination for all serve framework log calls.
// The first time this is called, it will enable logging to the logger.
// All log messages will be passed to the given structured logger, and in groupName.
// If groupName is empty, a groupName of "serve" will be used by default
func SetLogger(l *slog.Logger, groupName string) {
	if groupName == "" {
		groupName = "serve"
	}
	logger = l.WithGroup(groupName)
}

// Error sends an error to the logger.
// Error will send to the given logger if present, or the default logger.
// In other words, Error will always send a message to the log if the logging level is set to include errors.
// If no logger was set, it will put the error in the "serve" group.
// It will put the error in the "module" subgroup if present.
func Error(ctx context.Context, module string, msg string, args ...any) {
	var l *slog.Logger
	if logger != nil {
		l = logger
	} else {
		l = slog.Default().WithGroup("serve")
	}
	if ctx != nil {
		ctx = context.Background()
	}
	if module != "" {
		l.ErrorContext(ctx, msg, slog.Group(module, args))
	} else {
		l.ErrorContext(ctx, msg, args)
	}
}

// Warn sends a warning to the logger.
// Warn will send to the given logger if present, or the default logger.
// In other words, Warn will always send a message to the log if the logging level is set to warning.
// If no logger was set, it will put the warning in the "serve" group and the "module" subgroup.
func Warn(ctx context.Context, module string, msg string, args ...any) {
	var l *slog.Logger
	if logger != nil {
		l = logger
	} else {
		l = slog.Default().WithGroup("serve")
	}
	if ctx != nil {
		ctx = context.Background()
	}
	if module != "" {
		l.WarnContext(ctx, msg, slog.Group(module, args))
	} else {
		l.WarnContext(ctx, msg, args)
	}
}

// Info sends an info message to the logger if one has been set.
//
// If ctx is nil, the background context will be used.
// Set module to the name of the area of the server being debugged, or empty to not include a subgroup.
func Info(ctx context.Context, module string, msg string, args ...any) {
	if logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if module != "" {
		logger.InfoContext(ctx, msg, slog.Group(module, args))
	} else {
		logger.InfoContext(ctx, msg, args)
	}
}

// Debug sends a debug message to the logger if one has been set.
//
// If ctx is nil, the background context will be used.
// Set module to the name of the area of the server being debugged.
func Debug(ctx context.Context, module string, msg string, args ...any) {
	if logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if module != "" {
		logger.DebugContext(ctx, msg, slog.Group(module, args))
	} else {
		logger.DebugContext(ctx, msg, args)
	}
}

func IsDebugging() bool {
	return logger != nil && logger.Enabled(context.Background(), slog.LevelDebug)
}
