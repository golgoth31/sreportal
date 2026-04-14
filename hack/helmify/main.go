// hack/helmify is a post-processor that runs helmify then patches the generated
// Helm chart to support optional auth configuration.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	if err := patchFile(valuesFile, injectAuthBlock); err != nil {
		return fmt.Errorf("patching values.yaml: %w", err)
	}

	if err := patchFile(valuesFile, injectFlowObserverBlock); err != nil {
		return fmt.Errorf("patching values.yaml (flowObserver): %w", err)
	}

	if err := patchFile(deploymentFile, makeHeaderAPIKeyConditional); err != nil {
		return fmt.Errorf("patching deployment.yaml: %w", err)
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
        {{- end }}`
)

const flowObserverBlock = `flowObserver:
  enabled: false
  name: flow-observer-main
  portalRef: main
  reconcileInterval: "5m"
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
