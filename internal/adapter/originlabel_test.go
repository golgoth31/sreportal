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

package adapter

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// captureSlog redirects slog.Default() to buf for the duration of the test.
func captureSlog(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func TestOriginRefV2FromLabel_Valid(t *testing.T) {
	got := originRefV2FromLabel("service/ns/name")
	if got == nil || got.Kind != "service" || got.Namespace != "ns" || got.Name != "name" {
		t.Fatalf("unexpected ref: %+v", got)
	}
}

func TestOriginRefV2FromLabel_EmptyIsSilent(t *testing.T) {
	buf := captureSlog(t, slog.LevelDebug)
	if got := originRefV2FromLabel(""); got != nil {
		t.Fatalf("want nil for empty label, got %+v", got)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty label must not log, got: %q", buf.String())
	}
}

func TestOriginRefV2FromLabel_MalformedLogsWarn(t *testing.T) {
	buf := captureSlog(t, slog.LevelWarn)
	if got := originRefV2FromLabel("not-a-valid-ref"); got != nil {
		t.Fatalf("want nil for malformed label, got %+v", got)
	}
	out := buf.String()
	if !strings.Contains(out, "level=WARN") ||
		!strings.Contains(out, "malformed external-dns resource label") {
		t.Fatalf("expected WARN log for malformed label, got: %q", out)
	}
}

// origin v1 shares parseOriginLabel; one case is enough to cover the v1 wrapper.
func TestOriginRefFromLabel_MalformedReturnsNil(t *testing.T) {
	_ = captureSlog(t, slog.LevelWarn)
	if got := originRefFromLabel("bad"); got != nil {
		t.Fatalf("want nil for malformed label, got %+v", got)
	}
}
