// hack/helmify is a post-processor that runs helmify then patches the generated
// Helm chart to support optional auth configuration and to keep the rendered
// manifests valid YAML.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	helmDir := filepath.Join(rootDir, "helm")
	valuesFile := filepath.Join(helmDir, "values.yaml")
	deploymentFile := filepath.Join(helmDir, "templates", "deployment.yaml")
	kustomize := filepath.Join(rootDir, "bin", "kustomize")
	helmifyBin := filepath.Join(rootDir, "bin", "helmify")

	if err := runHelmify(rootDir, kustomize, helmifyBin); err != nil {
		return fmt.Errorf("running helmify: %w", err)
	}

	if err := patchTemplates(filepath.Join(helmDir, "templates"), dedupeSelectorLabels); err != nil {
		return fmt.Errorf("patching chart templates (selector labels): %w", err)
	}

	if err := patchFile(valuesFile, injectAuthBlock); err != nil {
		return fmt.Errorf("patching values.yaml: %w", err)
	}

	if err := patchFile(valuesFile, injectFlowObserverBlock); err != nil {
		return fmt.Errorf("patching values.yaml (flowObserver): %w", err)
	}

	if err := patchFile(deploymentFile, makeHeaderAPIKeyConditional); err != nil {
		return fmt.Errorf("patching deployment.yaml: %w", err)
	}

	if err := patchFile(deploymentFile, injectExtraVolumes); err != nil {
		return fmt.Errorf("patching deployment.yaml (extraVolumes): %w", err)
	}

	if err := patchFile(valuesFile, injectExtraVolumesBlock); err != nil {
		return fmt.Errorf("patching values.yaml (extraVolumes): %w", err)
	}

	fmt.Println("helmify post-processing complete")

	return nil
}

func runHelmify(rootDir, kustomize, helmifyBin string) error {
	kustomizeCmd := exec.Command(kustomize, "build", filepath.Join(rootDir, "config", "default"))
	helmifyCmd := exec.Command(helmifyBin, "-generate-defaults", "helm")

	pipe, err := kustomizeCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}

	helmifyCmd.Stdin = pipe
	helmifyCmd.Stdout = os.Stdout
	helmifyCmd.Stderr = os.Stderr
	kustomizeCmd.Stderr = os.Stderr

	if err := kustomizeCmd.Start(); err != nil {
		return fmt.Errorf("starting kustomize: %w", err)
	}

	if err := helmifyCmd.Start(); err != nil {
		return fmt.Errorf("starting helmify: %w", err)
	}

	if err := kustomizeCmd.Wait(); err != nil {
		return fmt.Errorf("kustomize failed: %w", err)
	}

	if err := helmifyCmd.Wait(); err != nil {
		return fmt.Errorf("helmify failed: %w", err)
	}

	return nil
}

// patchFile reads a file, applies a transform function, and writes back the result.
// If the transform returns the input unchanged, the file is not rewritten.
func patchFile(path string, transform func(string) string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	original := string(data)
	patched := transform(original)

	if patched == original {
		return nil
	}

	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// patchTemplates applies a transform function to every chart template.
func patchTemplates(dir string, transform func(string) string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		if err := patchFile(filepath.Join(dir, entry.Name()), transform); err != nil {
			return err
		}
	}

	return nil
}

// selectorLabelsInclude matches the helm.selectorLabels helper call helmify writes in
// place of the labels it templated, capturing the helper's nindent argument.
var selectorLabelsInclude = regexp.MustCompile(`^\s*\{\{- include "helm\.selectorLabels" \. \| nindent (\d+) \}\}\s*$`)

// selectorLabelPrefixes are the labels the helm.selectorLabels helper emits. helmify
// appends the helper call but leaves the literal labels it replaced in place, so the
// rendered manifest carries the same mapping key twice. Strict parsers reject it -
// notably Flux's HelmRelease post-renderer, which fails the whole release with
// "mapping key \"app.kubernetes.io/name\" already defined".
var selectorLabelPrefixes = []string{
	"app.kubernetes.io/name:",
	"app.kubernetes.io/instance:",
}

// dedupeSelectorLabels drops the literal selector labels the helm.selectorLabels
// helper already emits at the same indentation, so each label is rendered once.
func dedupeSelectorLabels(content string) string {
	lines := strings.Split(content, "\n")
	dropped := make(map[int]bool)

	for i, line := range lines {
		match := selectorLabelsInclude.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		nindent, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		// The labels the helper replaces sit right above it, indented with nindent.
		// Walk back over that block and drop the labels the helper emits.
		for j := i - 1; j >= 0 && indentWidth(lines[j]) == nindent; j-- {
			if hasSelectorLabelPrefix(lines[j]) {
				dropped[j] = true
			}
		}
	}

	if len(dropped) == 0 {
		return content
	}

	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		if !dropped[i] {
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "\n")
}

func indentWidth(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func hasSelectorLabelPrefix(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range selectorLabelPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return false
}

const authBlock = `auth:
  enabled: false
  secretRef: ""
  secretKey: ""
`

// injectAuthBlock appends the auth configuration block to values.yaml
// if it is not already present.
func injectAuthBlock(content string) string {
	if strings.Contains(content, "\nauth:\n") || strings.HasPrefix(content, "auth:\n") {
		return content
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content + authBlock
}

const (
	oldEnvBlock = `        - name: HEADER_API_KEY
          valueFrom:
            secretKeyRef:
              key: HEADER_API_KEY
              name: secret`

	newEnvBlock = `        {{- if .Values.auth.enabled }}
        - name: HEADER_API_KEY
          valueFrom:
            secretKeyRef:
              key: {{ .Values.auth.secretKey | quote }}
              name: {{ .Values.auth.secretRef | quote }}
        {{- end }}
        - name: SREPORTAL_CONTROLLER_SA
          value: system:serviceaccount:{{ .Release.Namespace }}:{{ include "helm.serviceAccountName" . }}`
)

const flowObserverBlock = `flowObserver:
  enabled: false
  name: flow-observer-main
  portalRef: main
  reconcileInterval: "5m"
  evaluatedEdgeTypes:
  - service
  prometheus:
    address: "http://prometheus.internal"
    queryWindow: "5m"
  metrics: []
`

// injectFlowObserverBlock appends the flowObserver configuration block to values.yaml
// if it is not already present.
func injectFlowObserverBlock(content string) string {
	if strings.Contains(content, "\nflowObserver:\n") || strings.HasPrefix(content, "flowObserver:\n") {
		return content
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content + flowObserverBlock
}

// makeHeaderAPIKeyConditional replaces the hardcoded HEADER_API_KEY env var
// with a Helm conditional block using auth values.
func makeHeaderAPIKeyConditional(content string) string {
	return strings.Replace(content, oldEnvBlock, newEnvBlock, 1)
}

const extraVolumesValuesBlock = `extraVolumes: []
extraVolumeMounts: []
`

// injectExtraVolumesBlock appends the extraVolumes/extraVolumeMounts configuration block
// to values.yaml if it is not already present.
func injectExtraVolumesBlock(content string) string {
	if strings.Contains(content, "\nextraVolumes:") || strings.HasPrefix(content, "extraVolumes:") {
		return content
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content + extraVolumesValuesBlock
}

const (
	oldVolumeMountsEnd = `        - mountPath: /etc/sreportal
          name: operator-config
          readOnly: true
      nodeSelector:`

	newVolumeMountsEnd = `        - mountPath: /etc/sreportal
          name: operator-config
          readOnly: true
        {{- with .Values.extraVolumeMounts }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      nodeSelector:`

	oldVolumesEnd = `          name: {{ include "helm.fullname" . }}-config
        name: operator-config`

	newVolumesEnd = `          name: {{ include "helm.fullname" . }}-config
        name: operator-config
      {{- with .Values.extraVolumes }}
      {{- toYaml . | nindent 6 }}
      {{- end }}`
)

// injectExtraVolumes patches the deployment template to support extra volumes and volumeMounts
// from values.
func injectExtraVolumes(content string) string {
	content = strings.Replace(content, oldVolumeMountsEnd, newVolumeMountsEnd, 1)
	content = strings.Replace(content, oldVolumesEnd, newVolumesEnd, 1)

	return content
}
