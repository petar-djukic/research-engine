// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package container implements container runtime detection and execution.
// Implements: prd002-conversion R5.5-R5.10 (container runtime strategy);
//
//	docs/ARCHITECTURE § Conversion.
package container

import (
	"fmt"
	"io"
	"os/exec"
)

const (
	binDocker = "docker"
	binPodman = "podman"
)

// Runtime provides container operations: checking availability, verifying
// images, and running containers.
type Runtime interface {
	// Name returns the runtime name ("docker" or "podman").
	Name() string

	// Available reports whether the runtime binary exists on PATH and
	// responds to an info command.
	Available() bool

	// ImageExists checks whether the named image exists locally.
	// Returns nil when the image is found, or an error describing the failure.
	ImageExists(image string) error

	// Run executes a container with the given image, piping stdin and stdout.
	Run(image string, stdin io.Reader, stdout io.Writer) error
}

// executor abstracts command execution for testing.
type executor interface {
	LookPath(file string) (string, error)
	RunSilent(name string, args ...string) error
	RunPiped(name string, args []string, stdin io.Reader, stdout io.Writer) error
}

// osExecutor is the production executor backed by os/exec.
type osExecutor struct{}

func (o *osExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (o *osExecutor) RunSilent(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (o *osExecutor) RunPiped(name string, args []string, stdin io.Reader, stdout io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	return cmd.Run()
}

// runtime implements Runtime for a specific container binary. Both Docker
// and Podman share the same logic; they differ only in binary name and the
// subcommand used to check image existence.
type runtime struct {
	bin           string
	imageCheckCmd []string // e.g. ["image", "inspect"] for docker
	exec          executor
}

func (r *runtime) Name() string { return r.bin }

func (r *runtime) Available() bool {
	if _, err := r.exec.LookPath(r.bin); err != nil {
		return false
	}
	return r.exec.RunSilent(r.bin, "info") == nil
}

func (r *runtime) ImageExists(image string) error {
	args := make([]string, 0, len(r.imageCheckCmd)+1)
	args = append(args, r.imageCheckCmd...)
	args = append(args, image)

	if err := r.exec.RunSilent(r.bin, args...); err != nil {
		return fmt.Errorf("image %s not found in %s: %w", image, r.bin, err)
	}
	return nil
}

func (r *runtime) Run(image string, stdin io.Reader, stdout io.Writer) error {
	args := []string{"run", "--rm", "-i", image}
	if err := r.exec.RunPiped(r.bin, args, stdin, stdout); err != nil {
		return fmt.Errorf("running %s container %s: %w", r.bin, image, err)
	}
	return nil
}

func newDockerRuntime(exec executor) *runtime {
	return &runtime{
		bin:           binDocker,
		imageCheckCmd: []string{"image", "inspect"},
		exec:          exec,
	}
}

func newPodmanRuntime(exec executor) *runtime {
	return &runtime{
		bin:           binPodman,
		imageCheckCmd: []string{"image", "exists"},
		exec:          exec,
	}
}

var defaultExec = &osExecutor{}

// DetectRuntime tries docker first, falls back to podman. Returns an error
// if neither runtime is available.
func DetectRuntime() (Runtime, error) {
	return detectRuntime(defaultExec)
}

func detectRuntime(exec executor) (Runtime, error) {
	docker := newDockerRuntime(exec)
	if docker.Available() {
		return docker, nil
	}

	podman := newPodmanRuntime(exec)
	if podman.Available() {
		return podman, nil
	}

	return nil, fmt.Errorf(
		"no container runtime available: neither %s nor %s found or operational",
		binDocker, binPodman,
	)
}
