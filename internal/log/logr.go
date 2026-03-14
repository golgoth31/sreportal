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

package log

import (
	"context"
	"log/slog"

	"github.com/go-logr/logr"
)

// ToLogr returns a logr.Logger backed by this Logger's underlying slog.Handler.
// Use this to bridge into controller-runtime: ctrl.SetLogger(log.Default().ToLogr()).
func (l *Logger) ToLogr() logr.Logger {
	return logr.FromSlogHandler(l.Logger.Handler())
}

// FromContext extracts the logr.Logger that controller-runtime injected into ctx,
// converts it to an slog.Logger, and wraps it in our Logger.
// Falls back to Default() when ctx has no logger.
func FromContext(ctx context.Context, keysAndValues ...any) *Logger {
	logrLogger, err := logr.FromContext(ctx)
	if err != nil {
		return Default()
	}

	if len(keysAndValues) > 0 {
		logrLogger = logrLogger.WithValues(keysAndValues...)
	}

	return &Logger{Logger: slog.New(logr.ToSlogHandler(logrLogger))}
}

// IntoContext stores the Logger into ctx for later retrieval by FromContext.
// The logr.Logger derived from our slog handler is stored so both logr.FromContext
// and log.FromContext callers see the same logger.
func IntoContext(ctx context.Context, l *Logger) context.Context {
	return logr.NewContext(ctx, l.ToLogr())
}
