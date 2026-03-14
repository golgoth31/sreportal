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

package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applog "github.com/golgoth31/sreportal/internal/log"
)

func TestToLogr_ReturnsValidLogrLogger(t *testing.T) {
	// Arrange
	logger := applog.Default()

	// Act
	logrLogger := logger.ToLogr()

	// Assert — a valid logr.Logger must have a non-nil sink
	require.NotNil(t, logrLogger.GetSink(), "ToLogr must return a logger with a non-nil sink")
}

func TestFromContext_WithLogrLogger_ReturnsWrappedLogger(t *testing.T) {
	// Arrange — inject a logr.Logger into the context (like controller-runtime does)
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slogLogger := slog.New(handler)
	logrLogger := logr.FromSlogHandler(slogLogger.Handler())
	ctx := logr.NewContext(context.Background(), logrLogger)

	// Act
	logger := applog.FromContext(ctx)

	// Assert — must not be nil and must log to the same handler
	require.NotNil(t, logger, "FromContext must return a non-nil Logger")
	logger.Info("test message", "key", "value")
	assert.Contains(t, buf.String(), "test message")
	assert.Contains(t, buf.String(), "key")
	assert.Contains(t, buf.String(), "value")
}

func TestFromContext_WithKeysAndValues_AttachesFields(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slogLogger := slog.New(handler)
	logrLogger := logr.FromSlogHandler(slogLogger.Handler())
	ctx := logr.NewContext(context.Background(), logrLogger)

	// Act
	logger := applog.FromContext(ctx, "component", "test-component")
	logger.Info("hello")

	// Assert
	assert.Contains(t, buf.String(), "component")
	assert.Contains(t, buf.String(), "test-component")
}

func TestFromContext_WithEmptyContext_FallsBackToDefault(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Act
	logger := applog.FromContext(ctx)

	// Assert — must return Default(), never nil
	require.NotNil(t, logger, "FromContext must fall back to Default() on empty context")
}

func TestIntoContext_RoundTrips(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	original := applog.FromSlog(slog.New(handler)).WithName("round-trip")

	// Act — store and retrieve
	ctx := applog.IntoContext(context.Background(), original)
	retrieved := applog.FromContext(ctx)

	// Assert — the retrieved logger must log through the same handler
	retrieved.Info("round trip message")
	assert.Contains(t, buf.String(), "round trip message")
}

func TestToLogr_RoundTripWithControllerRuntime(t *testing.T) {
	// Arrange — simulate what controller-runtime does: store a logr in ctx
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	original := applog.FromSlog(slog.New(handler))

	// Act — convert to logr, store in ctx, then extract back
	logrLogger := original.ToLogr()
	ctx := logr.NewContext(context.Background(), logrLogger)
	retrieved := applog.FromContext(ctx)

	// Assert
	retrieved.Info("controller-runtime bridge")
	assert.Contains(t, buf.String(), "controller-runtime bridge")
}
