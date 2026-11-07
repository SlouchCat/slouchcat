package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Docker is everything the install flow asks of Docker. The flow is tested against a fake; the
// real one (dockerCLI) runs the docker CLI of the image.
type Docker interface {
	// Container inspects the container name.
	Container(ctx context.Context, name string) (Container, error)
	// ComposeImage returns the image reference compose resolves for service under p (with the
	// env files as they are on disk now).
	ComposeImage(ctx context.Context, p Project, service string) (string, error)
	// RepoDigests returns the repository digests ("repo@sha256:…") of the local image ref.
	RepoDigests(ctx context.Context, ref string) ([]string, error)
	// Compose runs `docker compose <project arguments> <args>`.
	Compose(ctx context.Context, p Project, args ...string) error
}

// Container is the part of `docker container inspect` the updater reads.
type Container struct {
	// Image is the reference the container was created from (Config.Image).
	Image string
	// Labels are the container labels; they include the labels of its image.
	Labels map[string]string
	// Running is State.Running.
	Running bool
	// Health is State.Health.Status ("starting", "healthy", "unhealthy"); empty when the
	// container has no health check.
	Health string
}

// Healthy reports whether the container runs and passes its health check; a container without
// a health check counts as healthy while it runs.
func (c Container) Healthy() bool {
	return c.Running && (c.Health == "" || c.Health == "healthy")
}

// Project is the compose setup of the running server container, read from its labels.
type Project struct {
	Name        string
	WorkingDir  string
	ConfigFiles []string
	EnvFiles    []string
}

// args returns the global compose arguments that reproduce the project.
func (p Project) args() []string {
	args := []string{"compose", "--project-name", p.Name, "--project-directory", p.WorkingDir}
	for _, f := range p.EnvFiles {
		args = append(args, "--env-file", f)
	}
	for _, f := range p.ConfigFiles {
		args = append(args, "-f", f)
	}
	return args
}

// dockerCLI runs the docker binary of the updater image (CLAUDE.md rule 3: exec.CommandContext
// with an argument list, never a shell; ADR 0024).
type dockerCLI struct {
	// output receives the combined output of a failed command (the caller logs it).
	output func(args []string, out []byte)
}

// maxOutput caps what a command may print before it counts as failed; compose pull prints a few
// kilobytes.
const maxOutput = 1 << 20

// run executes docker with args and returns its standard output. The environment is cut down to
// what the CLI needs, so compose interpolates only from the env files of the project, as it did
// on the host, and never from the updater's own variables.
func (d dockerCLI) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec // G204: fixed binary, argument list built from container labels checked against the deploy folder
	cmd.Env = dockerEnv(os.Getenv)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil && stdout.Len() > maxOutput {
		err = errors.New("output too large")
	}
	if err != nil {
		if d.output != nil {
			d.output(args, append(stdout.Bytes(), stderr.Bytes()...))
		}
		return nil, fmt.Errorf("docker %s: %w", args[0], err)
	}
	return stdout.Bytes(), nil
}

// dockerEnv is the environment of a docker call: the search path, the home folder for the CLI
// configuration and the Docker connection variables, if set.
func dockerEnv(getenv func(string) string) []string {
	env := []string{"PATH=" + getenv("PATH"), "HOME=" + getenv("HOME")}
	for _, k := range []string{"DOCKER_HOST", "DOCKER_CONFIG", "DOCKER_CERT_PATH", "DOCKER_TLS_VERIFY"} {
		if v := getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (d dockerCLI) Container(ctx context.Context, name string) (Container, error) {
	out, err := d.run(ctx, "container", "inspect", name)
	if err != nil {
		return Container{}, err
	}
	var list []struct {
		Config struct {
			Image  string
			Labels map[string]string
		}
		State struct {
			Running bool
			Health  *struct{ Status string }
		}
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return Container{}, fmt.Errorf("inspect %s: %w", name, err)
	}
	if len(list) != 1 {
		return Container{}, fmt.Errorf("inspect %s: %d results", name, len(list))
	}
	c := list[0]
	res := Container{Image: c.Config.Image, Labels: c.Config.Labels, Running: c.State.Running}
	if c.State.Health != nil {
		res.Health = c.State.Health.Status
	}
	return res, nil
}

func (d dockerCLI) ComposeImage(ctx context.Context, p Project, service string) (string, error) {
	// Not --images: it prints the image of every dependency too, one line each, and
	// compose.updater.yaml makes the server depend on the updater.
	out, err := d.run(ctx, append(p.args(), "config", "--format", "json", service)...)
	if err != nil {
		return "", err
	}
	return composeImage(out, service)
}

// composeImage reads services.<service>.image from the output of compose config --format json.
func composeImage(out []byte, service string) (string, error) {
	var cfg struct {
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		return "", fmt.Errorf("compose config: %w", err)
	}
	ref := strings.TrimSpace(cfg.Services[service].Image)
	if ref == "" || strings.ContainsAny(ref, " \t\r\n") {
		return "", fmt.Errorf("compose config: no image for the service %s", service)
	}
	return ref, nil
}

func (d dockerCLI) RepoDigests(ctx context.Context, ref string) ([]string, error) {
	out, err := d.run(ctx, "image", "inspect", ref)
	if err != nil {
		return nil, err
	}
	var list []struct{ RepoDigests []string }
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("image inspect: %w", err)
	}
	if len(list) != 1 {
		return nil, fmt.Errorf("image inspect: %d results", len(list))
	}
	return list[0].RepoDigests, nil
}

func (d dockerCLI) Compose(ctx context.Context, p Project, args ...string) error {
	_, err := d.run(ctx, append(p.args(), args...)...)
	return err
}
