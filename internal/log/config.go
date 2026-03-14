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
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// Format represents the log output encoding.
type Format string

const (
	// FormatRaw produces human-readable console output (key=value pairs).
	FormatRaw Format = "raw"
	// FormatJSON produces structured JSON output.
	FormatJSON Format = "json"
	// FormatJSONECS produces ECS-compliant JSON output (Elastic Common Schema).
	FormatJSONECS Format = "json-ecs"
)

// Level represents the minimum log level.
type Level string

const (
	LevelTraceValue Level = "trace"
	LevelDebugValue Level = "debug"
	LevelInfoValue  Level = "info"
	LevelWarnValue  Level = "warn"
	LevelErrorValue Level = "error"
)

var (
	// ErrInvalidFormat is returned when an unknown log format is requested.
	ErrInvalidFormat = errors.New("invalid log format")
	// ErrInvalidLevel is returned when an unknown log level is requested.
	ErrInvalidLevel = errors.New("invalid log level")
)

// ParseFormat converts a string to a Format value.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatRaw, FormatJSON, FormatJSONECS:
		return Format(s), nil
	default:
		return "", fmt.Errorf("%w: %q (valid: raw, json, json-ecs)", ErrInvalidFormat, s)
	}
}

// ParseLevel converts a string to a Level value.
func ParseLevel(s string) (Level, error) {
	switch Level(s) {
	case LevelTraceValue, LevelDebugValue, LevelInfoValue, LevelWarnValue, LevelErrorValue:
		return Level(s), nil
	default:
		return "", fmt.Errorf("%w: %q (valid: trace, debug, info, warn, error)", ErrInvalidLevel, s)
	}
}

// Config holds all settings for the log package.
type Config struct {
	// Format selects the output encoding: "raw", "json", or "json-ecs".
	Format Format
	// Level sets the minimum log level: "trace", "debug", "info", "warn", "error".
	Level Level
	// Output is the writer for log output. Defaults to os.Stderr when nil.
	Output io.Writer
	// AddCaller adds source file and line number to log entries when true.
	AddCaller bool
	// DevMode when true enables stack traces for Warn and Error. When false, no stack traces.
	DevMode bool
	// StacktraceLevel sets the level at which stack traces are recorded (optional).
	// When nil, behaviour is driven by DevMode: stack at Error in dev, disabled otherwise.
	StacktraceLevel *Level
}

// BindFlags registers --log-level and --log-format on the given flag set.
// The flags parse directly into the Config fields.
func (c *Config) BindFlags(fs *flag.FlagSet) {
	// Set defaults before binding
	if c.Format == "" {
		c.Format = FormatRaw
	}
	if c.Level == "" {
		c.Level = LevelInfoValue
	}

	fs.Var(&formatFlag{config: c}, "log-format",
		`Log output format: "raw" (console), "json" (structured), "json-ecs" (Elastic Common Schema).`)
	fs.Var(&levelFlag{config: c}, "log-level",
		`Minimum log level: "trace", "debug", "info", "warn", "error".`)
}

// Init initialises the global slog.Default() with a zap backend configured
// according to the given Config. Call this once from main after flag.Parse().
// It replaces InitZapHandler and the controller-runtime zap dependency.
func Init(cfg Config) error {
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}

	zapLevel := toZapLevel(cfg.Level)
	core, err := buildCore(cfg.Format, output, zapLevel)
	if err != nil {
		return err
	}

	opts := []zap.Option{}
	if cfg.AddCaller {
		opts = append(opts, zap.AddCaller())
	}

	// Stack traces: only in dev mode unless StacktraceLevel is explicitly set.
	stackLevel := zapcore.Level(127) // above Fatal → never record in production
	if cfg.StacktraceLevel != nil {
		stackLevel = toZapLevel(*cfg.StacktraceLevel)
	} else if cfg.DevMode {
		stackLevel = zapcore.ErrorLevel
	}
	opts = append(opts, zap.AddStacktrace(stackLevel))

	zapLogger := zap.New(core, opts...)

	// Wire into slog.Default via zapslog (same threshold: stack only in dev or when explicit).
	slogHandler := zapslog.NewHandler(zapLogger.Core(),
		zapslog.AddStacktraceAt(slog.Level(stackLevel)),
	)
	slog.SetDefault(slog.New(slogHandler))

	return nil
}

// buildCore creates a zapcore.Core for the given format.
func buildCore(format Format, output io.Writer, level zapcore.Level) (zapcore.Core, error) {
	writeSyncer := zapcore.AddSync(output)

	switch format {
	case FormatRaw:
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder := zapcore.NewConsoleEncoder(encoderCfg)
		return zapcore.NewCore(encoder, writeSyncer, level), nil

	case FormatJSON:
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder := zapcore.NewJSONEncoder(encoderCfg)
		return zapcore.NewCore(encoder, writeSyncer, level), nil

	case FormatJSONECS:
		encoderCfg := ecszap.NewDefaultEncoderConfig()
		core := ecszap.NewCore(encoderCfg, writeSyncer, level)
		return core, nil

	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidFormat, format)
	}
}

// toZapLevel maps our Level to a zapcore.Level.
func toZapLevel(l Level) zapcore.Level {
	switch l {
	case LevelTraceValue:
		// Trace maps to zap Debug-1 (more verbose than Debug).
		return zapcore.Level(-1)
	case LevelDebugValue:
		return zapcore.DebugLevel
	case LevelInfoValue:
		return zapcore.InfoLevel
	case LevelWarnValue:
		return zapcore.WarnLevel
	case LevelErrorValue:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// --- flag.Value implementations ---

type formatFlag struct {
	config *Config
}

func (f *formatFlag) String() string {
	if f.config == nil {
		return string(FormatRaw)
	}
	return string(f.config.Format)
}

func (f *formatFlag) Set(s string) error {
	v, err := ParseFormat(s)
	if err != nil {
		return err
	}
	f.config.Format = v
	return nil
}

type levelFlag struct {
	config *Config
}

func (f *levelFlag) String() string {
	if f.config == nil {
		return string(LevelInfoValue)
	}
	return string(f.config.Level)
}

func (f *levelFlag) Set(s string) error {
	v, err := ParseLevel(s)
	if err != nil {
		return err
	}
	f.config.Level = v
	return nil
}
