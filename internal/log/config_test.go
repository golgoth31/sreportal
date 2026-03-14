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
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applog "github.com/golgoth31/sreportal/internal/log"
)

// --- Format validation ---

func TestParseFormat_ValidValues(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  applog.Format
	}{
		{"raw", "raw", applog.FormatRaw},
		{"json", "json", applog.FormatJSON},
		{"json-ecs", "json-ecs", applog.FormatJSONECS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applog.ParseFormat(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseFormat_Invalid_ReturnsError(t *testing.T) {
	_, err := applog.ParseFormat("yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, applog.ErrInvalidFormat)
}

// --- Level validation ---

func TestParseLevel_ValidValues(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  applog.Level
	}{
		{"trace", "trace", applog.LevelTraceValue},
		{"debug", "debug", applog.LevelDebugValue},
		{"info", "info", applog.LevelInfoValue},
		{"warn", "warn", applog.LevelWarnValue},
		{"error", "error", applog.LevelErrorValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := applog.ParseLevel(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseLevel_Invalid_ReturnsError(t *testing.T) {
	_, err := applog.ParseLevel("verbose")
	require.Error(t, err)
	assert.ErrorIs(t, err, applog.ErrInvalidLevel)
}

// --- Init produces working loggers ---

func TestInit_RawFormat_ProducesConsoleOutput(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := applog.Config{
		Format: applog.FormatRaw,
		Level:  applog.LevelInfoValue,
		Output: &buf,
	}

	// Act
	err := applog.Init(cfg)
	require.NoError(t, err)
	applog.Default().Info("raw test message", "key", "val")

	// Assert — raw format uses console encoder: no JSON braces
	output := buf.String()
	assert.Contains(t, output, "raw test message")
	assert.NotContains(t, output, `"message"`, "raw format must not produce JSON")
}

func TestInit_JSONFormat_ProducesJSONOutput(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := applog.Config{
		Format: applog.FormatJSON,
		Level:  applog.LevelInfoValue,
		Output: &buf,
	}

	// Act
	err := applog.Init(cfg)
	require.NoError(t, err)
	applog.Default().Info("json test message", "count", 42)

	// Assert — must be valid JSON
	output := buf.String()
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m), "output must be valid JSON: %s", line)
	}
	assert.Contains(t, output, "json test message")
}

func TestInit_JSONECSFormat_ProducesECSFields(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := applog.Config{
		Format: applog.FormatJSONECS,
		Level:  applog.LevelInfoValue,
		Output: &buf,
	}

	// Act
	err := applog.Init(cfg)
	require.NoError(t, err)
	applog.Default().Info("ecs test message")

	// Assert — ECS format must contain ecs.version field
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.NotEmpty(t, lines)
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &m))
	assert.Contains(t, m, "ecs.version", "ECS format must include ecs.version field")
	assert.Contains(t, output, "ecs test message")
}

// --- Level filtering ---

func TestInit_LevelFiltering_DebugNotShownAtInfoLevel(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := applog.Config{
		Format: applog.FormatRaw,
		Level:  applog.LevelInfoValue,
		Output: &buf,
	}

	// Act
	err := applog.Init(cfg)
	require.NoError(t, err)
	applog.Default().Debug("should be hidden")
	applog.Default().Info("should be visible")

	// Assert
	output := buf.String()
	assert.NotContains(t, output, "should be hidden")
	assert.Contains(t, output, "should be visible")
}

func TestInit_LevelFiltering_DebugShownAtDebugLevel(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	cfg := applog.Config{
		Format: applog.FormatRaw,
		Level:  applog.LevelDebugValue,
		Output: &buf,
	}

	// Act
	err := applog.Init(cfg)
	require.NoError(t, err)
	applog.Default().Debug("debug message visible")

	// Assert
	assert.Contains(t, buf.String(), "debug message visible")
}

// --- BindFlags ---

func TestBindFlags_RegistersFlagsOnFlagSet(t *testing.T) {
	// Arrange
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var cfg applog.Config

	// Act
	cfg.BindFlags(fs)

	// Assert — flags must be registered
	require.NotNil(t, fs.Lookup("log-level"), "must register --log-level")
	require.NotNil(t, fs.Lookup("log-format"), "must register --log-format")
}

func TestBindFlags_ParsesValues(t *testing.T) {
	// Arrange
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var cfg applog.Config
	cfg.BindFlags(fs)

	// Act
	err := fs.Parse([]string{"--log-level=debug", "--log-format=json"})
	require.NoError(t, err)

	// Assert
	assert.Equal(t, applog.LevelDebugValue, cfg.Level)
	assert.Equal(t, applog.FormatJSON, cfg.Format)
}

func TestBindFlags_Defaults(t *testing.T) {
	// Arrange
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var cfg applog.Config
	cfg.BindFlags(fs)

	// Act — parse with no args, should use defaults
	err := fs.Parse([]string{})
	require.NoError(t, err)

	// Assert
	assert.Equal(t, applog.LevelInfoValue, cfg.Level)
	assert.Equal(t, applog.FormatRaw, cfg.Format)
}

func TestBindFlags_InvalidFormat_ReturnsError(t *testing.T) {
	// Arrange
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	var cfg applog.Config
	cfg.BindFlags(fs)

	// Act
	err := fs.Parse([]string{"--log-format=yaml"})

	// Assert
	require.Error(t, err)
}

func TestBindFlags_InvalidLevel_ReturnsError(t *testing.T) {
	// Arrange
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	var cfg applog.Config
	cfg.BindFlags(fs)

	// Act
	err := fs.Parse([]string{"--log-level=verbose"})

	// Assert
	require.Error(t, err)
}
