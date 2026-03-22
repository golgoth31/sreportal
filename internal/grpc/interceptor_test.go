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

package grpc_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalgrpc "github.com/golgoth31/sreportal/internal/grpc"
)

func TestLoggingInterceptor_WhenHandlerReturnsError_LogsWarning(t *testing.T) {
	handler := &logRecordHandler{}
	slog.SetDefault(slog.New(handler))

	interceptor := internalgrpc.LoggingInterceptor()

	handlerErr := connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry is required"))
	next := func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, handlerErr
	}

	req := connect.NewRequest[any](nil)
	_, err := interceptor(next)(context.Background(), req)

	require.ErrorIs(t, err, handlerErr)
	require.Len(t, handler.records, 1)
	assert.Equal(t, slog.LevelWarn, handler.records[0].Level)
	assert.Equal(t, "request error", handler.records[0].Message)
}

func TestLoggingInterceptor_WhenHandlerSucceeds_DoesNotLog(t *testing.T) {
	handler := &logRecordHandler{}
	slog.SetDefault(slog.New(handler))

	interceptor := internalgrpc.LoggingInterceptor()

	next := func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse[any](nil), nil
	}

	req := connect.NewRequest[any](nil)
	_, err := interceptor(next)(context.Background(), req)

	require.NoError(t, err)
	assert.Empty(t, handler.records)
}

// logRecordHandler captures slog records for assertion.
type logRecordHandler struct {
	records []slog.Record
}

func (h *logRecordHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *logRecordHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *logRecordHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *logRecordHandler) WithGroup(_ string) slog.Handler      { return h }
