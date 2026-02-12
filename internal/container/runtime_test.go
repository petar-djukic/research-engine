// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package container

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// mockExecutor records calls and returns configured responses.
type mockExecutor struct {
	availableBins map[string]bool // binary -> whether LookPath succeeds
	runnableCmds  map[string]bool // "bin arg1 arg2" -> whether RunSilent succeeds
	runPipedFunc  func(name string, args []string, stdin io.Reader, stdout io.Writer) error
}

func (m *mockExecutor) LookPath(file string) (string, error) {
	if m.availableBins[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New("not found: " + file)
}

func (m *mockExecutor) RunSilent(name string, args ...string) error {
	key := name + " " + strings.Join(args, " ")
	if m.runnableCmds[key] {
		return nil
	}
	return errors.New("command failed: " + key)
}

func (m *mockExecutor) RunPiped(name string, args []string, stdin io.Reader, stdout io.Writer) error {
	if m.runPipedFunc != nil {
		return m.runPipedFunc(name, args, stdin, stdout)
	}
	return nil
}

func TestDetectRuntime(t *testing.T) {
	tests := []struct {
		name     string
		exec     *mockExecutor
		wantName string
		wantErr  bool
	}{
		{
			name: "docker available",
			exec: &mockExecutor{
				availableBins: map[string]bool{"docker": true},
				runnableCmds:  map[string]bool{"docker info": true},
			},
			wantName: "docker",
		},
		{
			name: "podman fallback when docker missing",
			exec: &mockExecutor{
				availableBins: map[string]bool{"podman": true},
				runnableCmds:  map[string]bool{"podman info": true},
			},
			wantName: "podman",
		},
		{
			name: "neither available",
			exec: &mockExecutor{
				availableBins: map[string]bool{},
				runnableCmds:  map[string]bool{},
			},
			wantErr: true,
		},
		{
			name: "docker on PATH but info fails, podman works",
			exec: &mockExecutor{
				availableBins: map[string]bool{"docker": true, "podman": true},
				runnableCmds:  map[string]bool{"podman info": true},
			},
			wantName: "podman",
		},
		{
			name: "both available, docker preferred",
			exec: &mockExecutor{
				availableBins: map[string]bool{"docker": true, "podman": true},
				runnableCmds:  map[string]bool{"docker info": true, "podman info": true},
			},
			wantName: "docker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := detectRuntime(tt.exec)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "no container runtime available") {
					t.Errorf("error should mention no runtime available, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rt.Name() != tt.wantName {
				t.Errorf("got runtime %q, want %q", rt.Name(), tt.wantName)
			}
		})
	}
}

func TestImageExists(t *testing.T) {
	tests := []struct {
		name    string
		mkRT    func(*mockExecutor) Runtime
		image   string
		cmds    map[string]bool
		wantErr bool
	}{
		{
			name:  "docker image exists",
			mkRT:  func(e *mockExecutor) Runtime { return newDockerRuntime(e) },
			image: "markitdown:latest",
			cmds:  map[string]bool{"docker image inspect markitdown:latest": true},
		},
		{
			name:    "docker image not found",
			mkRT:    func(e *mockExecutor) Runtime { return newDockerRuntime(e) },
			image:   "markitdown:latest",
			cmds:    map[string]bool{},
			wantErr: true,
		},
		{
			name:  "podman image exists",
			mkRT:  func(e *mockExecutor) Runtime { return newPodmanRuntime(e) },
			image: "markitdown:latest",
			cmds:  map[string]bool{"podman image exists markitdown:latest": true},
		},
		{
			name:    "podman image not found",
			mkRT:    func(e *mockExecutor) Runtime { return newPodmanRuntime(e) },
			image:   "markitdown:latest",
			cmds:    map[string]bool{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{runnableCmds: tt.cmds}
			rt := tt.mkRT(exec)
			err := rt.ImageExists(tt.image)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.image) {
					t.Errorf("error should mention image name, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		mkRT     func(*mockExecutor) Runtime
		image    string
		input    string
		pipeFunc func(string, []string, io.Reader, io.Writer) error
		wantOut  string
		wantErr  bool
	}{
		{
			name:  "docker run pipes stdin to stdout",
			mkRT:  func(e *mockExecutor) Runtime { return newDockerRuntime(e) },
			image: "markitdown:latest",
			input: "pdf content",
			pipeFunc: func(name string, args []string, stdin io.Reader, stdout io.Writer) error {
				if name != "docker" {
					return errors.New("expected docker binary")
				}
				data, _ := io.ReadAll(stdin)
				_, _ = stdout.Write([]byte("converted: " + string(data)))
				return nil
			},
			wantOut: "converted: pdf content",
		},
		{
			name:  "podman run pipes stdin to stdout",
			mkRT:  func(e *mockExecutor) Runtime { return newPodmanRuntime(e) },
			image: "markitdown:latest",
			input: "pdf content",
			pipeFunc: func(name string, args []string, stdin io.Reader, stdout io.Writer) error {
				if name != "podman" {
					return errors.New("expected podman binary")
				}
				data, _ := io.ReadAll(stdin)
				_, _ = stdout.Write([]byte("converted: " + string(data)))
				return nil
			},
			wantOut: "converted: pdf content",
		},
		{
			name:  "run failure returns wrapped error",
			mkRT:  func(e *mockExecutor) Runtime { return newDockerRuntime(e) },
			image: "markitdown:latest",
			pipeFunc: func(string, []string, io.Reader, io.Writer) error {
				return errors.New("container exited with code 1")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockExecutor{runPipedFunc: tt.pipeFunc}
			rt := tt.mkRT(exec)
			var out bytes.Buffer
			err := rt.Run(tt.image, strings.NewReader(tt.input), &out)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := out.String(); got != tt.wantOut {
				t.Errorf("got output %q, want %q", got, tt.wantOut)
			}
		})
	}
}

func TestRuntimeName(t *testing.T) {
	exec := &mockExecutor{}
	docker := newDockerRuntime(exec)
	if docker.Name() != "docker" {
		t.Errorf("docker runtime name = %q, want %q", docker.Name(), "docker")
	}
	podman := newPodmanRuntime(exec)
	if podman.Name() != "podman" {
		t.Errorf("podman runtime name = %q, want %q", podman.Name(), "podman")
	}
}
