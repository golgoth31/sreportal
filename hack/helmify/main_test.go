package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInjectAuthBlock_WhenAbsent_AppendsBlock(t *testing.T) {
	input := "kubernetesClusterDomain: cluster.local\n"

	got := injectAuthBlock(input)

	assert.Contains(t, got, "auth:\n")
	assert.Contains(t, got, "  enabled: false\n")
	assert.Contains(t, got, "  secretRef: \"\"\n")
	assert.Contains(t, got, "  secretKey: \"\"\n")
	assert.True(t, len(got) > len(input))
}

func TestInjectAuthBlock_WhenAlreadyPresent_ReturnsUnchanged(t *testing.T) {
	input := "kubernetesClusterDomain: cluster.local\nauth:\n  enabled: true\n"

	got := injectAuthBlock(input)

	assert.Equal(t, input, got)
}

func TestInjectAuthBlock_WhenAuthAtStart_ReturnsUnchanged(t *testing.T) {
	input := "auth:\n  enabled: false\n"

	got := injectAuthBlock(input)

	assert.Equal(t, input, got)
}

func TestInjectAuthBlock_WhenNoTrailingNewline_AddsNewlineBeforeBlock(t *testing.T) {
	input := "foo: bar"

	got := injectAuthBlock(input)

	assert.Contains(t, got, "foo: bar\nauth:\n")
}

func TestMakeHeaderAPIKeyConditional_WhenBlockPresent_ReplacesWithConditional(t *testing.T) {
	input := `        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: HEADER_API_KEY
          valueFrom:
            secretKeyRef:
              key: HEADER_API_KEY
              name: secret
        - name: KUBERNETES_CLUSTER_DOMAIN
`

	got := makeHeaderAPIKeyConditional(input)

	assert.NotContains(t, got, "name: secret")
	assert.Contains(t, got, "{{- if .Values.auth.enabled }}")
	assert.Contains(t, got, "{{ .Values.auth.secretRef | quote }}")
	assert.Contains(t, got, "{{ .Values.auth.secretKey | quote }}")
	assert.Contains(t, got, "{{- end }}")
	// Surrounding env vars are preserved
	assert.Contains(t, got, "POD_NAMESPACE")
	assert.Contains(t, got, "KUBERNETES_CLUSTER_DOMAIN")
}

func TestMakeHeaderAPIKeyConditional_WhenBlockAbsent_ReturnsUnchanged(t *testing.T) {
	input := `        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
`

	got := makeHeaderAPIKeyConditional(input)

	assert.Equal(t, input, got)
}

func TestMakeHeaderAPIKeyConditional_WhenAlreadyPatched_ReturnsUnchanged(t *testing.T) {
	input := `        {{- if .Values.auth.enabled }}
        - name: HEADER_API_KEY
          valueFrom:
            secretKeyRef:
              key: {{ .Values.auth.secretKey | quote }}
              name: {{ .Values.auth.secretRef | quote }}
        {{- end }}
`

	got := makeHeaderAPIKeyConditional(input)

	assert.Equal(t, input, got)
}

func TestInjectExtraVolumesBlock_WhenAbsent_AppendsBlock(t *testing.T) {
	input := "auth:\n  enabled: false\n"

	got := injectExtraVolumesBlock(input)

	assert.Contains(t, got, "extraVolumes: []")
	assert.Contains(t, got, "extraVolumeMounts: []")
	assert.True(t, len(got) > len(input))
}

func TestInjectExtraVolumesBlock_WhenAlreadyPresent_ReturnsUnchanged(t *testing.T) {
	input := "auth:\n  enabled: false\nextraVolumes: []\n"

	got := injectExtraVolumesBlock(input)

	assert.Equal(t, input, got)
}

func TestInjectExtraVolumes_WhenBlockPresent_InjectsDirectives(t *testing.T) {
	input := `        volumeMounts:
        - mountPath: /tmp/k8s-webhook-server/serving-certs
          name: webhook-certs
          readOnly: true
        - mountPath: /etc/sreportal
          name: operator-config
          readOnly: true
      nodeSelector: {}
      volumes:
      - name: webhook-certs
        secret:
          secretName: webhook-server-cert
      - configMap:
          items:
          - key: config.yaml
            path: config.yaml
          name: {{ include "helm.fullname" . }}-config
        name: operator-config
`

	got := injectExtraVolumes(input)

	assert.Contains(t, got, "{{- with .Values.extraVolumeMounts }}")
	assert.Contains(t, got, "{{- with .Values.extraVolumes }}")
}

func TestInjectExtraVolumes_WhenAlreadyPatched_ReturnsUnchanged(t *testing.T) {
	input := `        {{- with .Values.extraVolumeMounts }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      {{- with .Values.extraVolumes }}
      {{- toYaml . | nindent 6 }}
      {{- end }}
`

	got := injectExtraVolumes(input)

	assert.Equal(t, input, got)
}
