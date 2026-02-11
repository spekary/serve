// Package log controls how logging
package log

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/goradd/goradd/pkg/log"
	"github.com/goradd/serve/config"
)

// MaxErrorStackDepth is the maximum stack depth reported to the error log when a panic happens.
var MaxErrorStackDepth = 20

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
		args = append([]interface{}{slog.String("module", module)}, args...)
	}
	l.ErrorContext(ctx, msg, args...)
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
		args = append([]interface{}{slog.String("module", module)}, args...)
	}

	l.WarnContext(ctx, msg, args...)
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
		args = append([]interface{}{slog.String("module", module)}, args...)
	}

	logger.InfoContext(ctx, msg, args...)
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
		args = append([]interface{}{slog.String("module", module)}, args...)
	}

	logger.DebugContext(ctx, msg, args...)
}

func IsDebugging() bool {
	return logger != nil && logger.Enabled(context.Background(), slog.LevelDebug)
}

// StackTraceHandler is a log handler that automatically insert a stack trace for all
// Error level logs if not already present.
type StackTraceHandler struct {
	slog.Handler
}

func (h *StackTraceHandler) Handle(ctx context.Context, r slog.Record) error {
	var hasTrace bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "trace" {
			hasTrace = true
			return false
		}
		return true
	})
	if !hasTrace && r.Level >= slog.LevelError { // Only attach stack trace to ERROR level logs
		r.AddAttrs(slog.String("trace", log.StackTrace(1, MaxErrorStackDepth)))
	}
	return h.Handler.Handle(ctx, r)
}

type DevHandler struct {
	opts  slog.HandlerOptions
	attrs []slog.Attr // Persistent attributes from WithAttrs
	group string      // Current group prefix from WithGroup
}

func (h *DevHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *DevHandler) Handle(_ context.Context, r slog.Record) error {
	var printableAttrs []slog.Attr
	var trace string
	var module string

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "trace":
			trace = a.Value.String()
		case "module":
			module = a.Value.String()
		default:
			printableAttrs = append(printableAttrs, a)
		}
		return true
	})

	// 1. Format the basic line (Time Level Message)
	levelStr := r.Level.String()
	if r.Level == slog.LevelError {
		levelStr = "\033[31mERROR\033[0m" // Red color for errors
	}
	if h.group != "" {
		if module != "" {
			module = h.group + "." + module
		} else {
			module = h.group
		}
	}

	fmt.Printf("[%s] %s  %s: %s\n", r.Time.Format("15:04:05"), levelStr, module, r.Message)

	// 2. Print attributes (not stack)
	for _, a := range h.attrs { // permanent attributes
		h.printAttr(a)
	}
	for _, a := range printableAttrs {
		h.printAttr(a)
	}
	if trace != "" {
		fmt.Printf("Stack Trace:\n%s\n", trace)
	}

	return nil
}
func (h *DevHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DevHandler{
		opts:  h.opts,
		group: h.group,
		attrs: append(h.attrs, attrs...),
	}
}

func (h *DevHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &DevHandler{
		opts:  h.opts,
		attrs: h.attrs,
		group: newGroup,
	}
}

// Helper to handle group prefixing
func (h *DevHandler) printAttr(a slog.Attr) {

	fmt.Printf("\t%s:\t%v\n", a.Key, a.Value)
}

func init() {
	// Set up the default logger for development that will pretty print errors to the console.
	if !config.Release {
		level := slog.LevelInfo
		if config.Debug {
			level = slog.LevelDebug
		}
		handler := &DevHandler{opts: slog.HandlerOptions{Level: level}}
		logger = slog.New(&StackTraceHandler{handler}).WithGroup("serve")
	}
}
