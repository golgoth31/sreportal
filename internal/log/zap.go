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
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

// Deprecated: InitZapHandler is superseded by Init(Config). Use Init instead.
//
// InitZapHandler sets slog.Default() to use the given zap logger as backend
// via the zapslog handler (https://github.com/uber-go/zap/tree/master/exp/zapslog).
// When addStackInDev is true, stack traces are added for Error level and above; when false,
// stack traces are not added for Error (only for higher levels), so production logs stay clean.
func InitZapHandler(zapLogger *zap.Logger, addStackInDev bool) {
	if zapLogger == nil {
		return
	}
	stackLevel := slog.Level(999) // no stack for Error in production
	if addStackInDev {
		stackLevel = slog.LevelError
	}
	handler := zapslog.NewHandler(zapLogger.Core(), zapslog.AddStacktraceAt(stackLevel))
	slog.SetDefault(slog.New(handler))
}
