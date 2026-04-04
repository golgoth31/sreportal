// hack/helmify is a post-processor that runs helmify on the kustomize output.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	kustomize := filepath.Join(rootDir, "bin", "kustomize")
	helmifyBin := filepath.Join(rootDir, "bin", "helmify")

	if err := runHelmify(rootDir, kustomize, helmifyBin); err != nil {
		return fmt.Errorf("running helmify: %w", err)
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
