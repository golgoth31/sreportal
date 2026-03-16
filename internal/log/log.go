/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package log provides a structured logger built on log/slog with zap as the backend.
// Call log.Init(cfg) from main to configure format (raw/json/json-ecs) and level.
// All standard levels are supported: Trace, Debug, Info, Warn, Error, Fatal.
package log

import (
	"context"
	"log/slog"
	"os"
)

// LevelTrace is a custom level for trace (more verbose than Debug).
// When using the zapslog handler it is typically mapped to zap.DebugLevel.
const LevelTrace = slog.Level(-8)

// Logger wraps slog.Logger and exposes Trace, Debug, Info, Warn, Error, Fatal.
type Logger struct {
	*slog.Logger
}

// Default returns a Logger that uses slog.Default().
// Ensure log.Init(cfg) has been called from main so the default uses the configured backend.
func Default() *Logger {
	return &Logger{Logger: slog.Default()}
}

// FromSlog wraps an existing *slog.Logger.
func FromSlog(l *slog.Logger) *Logger {
	if l == nil {
		return &Logger{Logger: slog.Default()}
	}
	return &Logger{Logger: l}
}

// Slog returns the underlying *slog.Logger for use with APIs that require it.
func (l *Logger) Slog() *slog.Logger {
	return l.Logger
}

// WithName returns a new Logger with the given name as a group (for structured output).
func (l *Logger) WithName(name string) *Logger {
	return &Logger{Logger: l.WithGroup(name)}
}

// With returns a new Logger with the given key-value pairs attached.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...)}
}

// WithValues is an alias for With for compatibility with logr-style APIs.
func (l *Logger) WithValues(keysAndValues ...any) *Logger {
	return l.With(keysAndValues...)
}

// Trace logs at trace level (most verbose).
func (l *Logger) Trace(msg string, args ...any) {
	l.Log(context.Background(), LevelTrace, msg, args...)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs at warning level.
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs at error level. If err is non-nil, pass it as first arg: Error(err, "msg", ...).
func (l *Logger) Error(err error, msg string, args ...any) {
	if err != nil {
		args = append([]any{"err", err}, args...)
	}
	l.Logger.Error(msg, args...)
}

// Fatal logs at error level and exits the process with os.Exit(1).
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(msg, args...)
	os.Exit(1)
}

// Log emits a log record at the given level (e.g. LevelTrace, slog.LevelDebug).
func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.Logger.Log(ctx, level, msg, args...)
}

// VerboseLogger wraps a Logger with a verbosity level for V-style logging.
// V(0) maps to Info, V(1) maps to Debug, V(2+) maps to Trace.
type VerboseLogger struct {
	logger *Logger
	level  slog.Level
}

// V returns a VerboseLogger at the given verbosity level.
// V(0) = Info, V(1) = Debug, V(2+) = Trace.
// This provides compatibility with the logr.Logger.V() pattern used by controller-runtime.
func (l *Logger) V(verbosity int) *VerboseLogger {
	var level slog.Level
	switch {
	case verbosity <= 0:
		level = slog.LevelInfo
	case verbosity == 1:
		level = slog.LevelDebug
	default:
		level = LevelTrace
	}
	return &VerboseLogger{logger: l, level: level}
}

// Info logs at the verbosity level set by V().
func (v *VerboseLogger) Info(msg string, args ...any) {
	v.logger.Log(context.Background(), v.level, msg, args...)
}
