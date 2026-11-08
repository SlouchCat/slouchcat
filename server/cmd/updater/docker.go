package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/logging"
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
	// Compose runs `docker compose <project arguments> <args>`. A failed call returns a
	// *commandError with the last line docker printed to stderr.
	Compose(ctx context.Context, p Project, args ...string) error
	// Images lists the local images of the repository (`docker image ls`).
	Images(ctx context.Context, repository string) ([]Image, error)
	// RemoveImage runs `docker image rm ref`, never with --force: an image in use stays.
	RemoveImage(ctx context.Context, ref string) error
	// Run runs `docker run <args>` (the helper container of a self-update).
	Run(ctx context.Context, args ...string) error
	// RemoveContainer runs `docker container rm [--force] name`.
	RemoveContainer(ctx context.Context, name string, force bool) error
	// Logs returns the last tail lines the container name printed, redacted (redactLines).
	Logs(ctx context.Context, name string, tail int) ([]string, error)
}

// Image is one line of `docker image ls`; Tag and Digest are "<none>" when the image has none.
type Image struct {
	ID, Repository, Tag, Digest string
}

// Container is the part of `docker container inspect` the updater reads.
type Container struct {
	// Image is the reference the container was created from (Config.Image).
	Image string
	// ImageID is the ID of the image the container runs (Image of the inspect output).
	ImageID string
	// Labels are the container labels; they include the labels of its image.
	Labels map[string]string
	// Running is State.Running.
	Running bool
	// Health is State.Health.Status ("starting", "healthy", "unhealthy"); empty when the
	// container has no health check.
	Health string
	// Status is State.Status ("created", "running", "restarting", "exited", "dead", …).
	Status string
	// ExitCode is State.ExitCode, RestartCount the restarts since the container was created.
	ExitCode, RestartCount int
	// RestartPolicy and MaxRetries are HostConfig.RestartPolicy.Name and .MaximumRetryCount.
	RestartPolicy string
	MaxRetries    int
}

// Healthy reports whether the container runs and passes its health check; a container without
// a health check counts as healthy while it runs.
func (c Container) Healthy() bool {
	return c.Running && (c.Health == "" || c.Health == "healthy")
}

// maxRestarts is how often a new container may restart before the updater stops waiting for it.
const maxRestarts = 3

// GaveUp reports whether waiting for the container is pointless: it does not run, and it has
// restarted maxRestarts times already (a crash loop), or it is dead, or it exited and its restart
// policy will not start it again.
func (c Container) GaveUp() bool {
	if c.Running {
		return false
	}
	if c.RestartCount >= maxRestarts || c.Status == "dead" {
		return true
	}
	return c.Status == "exited" && !c.restarts()
}

// restarts reports whether the restart policy starts the exited container again.
func (c Container) restarts() bool {
	switch c.RestartPolicy {
	case "always", "unless-stopped":
		return true
	case "on-failure":
		return c.ExitCode != 0 && (c.MaxRetries == 0 || c.RestartCount < c.MaxRetries)
	}
	return false
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
	// failed receives a failed command: its arguments, the error (exit status) and the last line
	// of its output with every KEY=value redacted (lastLine); the caller logs it.
	failed func(args []string, err error, last string)
}

// maxOutput caps what a command may print before it counts as failed; compose pull prints a few
// kilobytes.
const maxOutput = 1 << 20

// run executes docker with args and returns its standard output. The environment is cut down to
// what the CLI needs, so compose interpolates only from the env files of the project, as it did
// on the host, and never from the updater's own variables.
func (d dockerCLI) run(ctx context.Context, args ...string) ([]byte, error) {
	stdout, _, err := d.output(ctx, args...)
	return stdout, err
}

// output is run with the standard error output too.
func (d dockerCLI) output(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec // G204: fixed binary, argument list built from container labels checked against the deploy folder
	cmd.Env = dockerEnv(os.Getenv)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	err = cmd.Run()
	if err == nil && (outBuf.Len() > maxOutput || errBuf.Len() > maxOutput) {
		err = errors.New("output too large")
	}
	if err != nil {
		if d.failed != nil {
			out := errBuf.Bytes()
			if len(bytes.TrimSpace(out)) == 0 {
				out = outBuf.Bytes()
			}
			d.failed(args, err, lastLine(out))
		}
		return nil, nil, newCommandError(args[0], err, errBuf.Bytes())
	}
	return outBuf.Bytes(), errBuf.Bytes(), nil
}

// maxErrorLine caps the stderr line a commandError carries; it ends up in the message of the
// install (ADR 0024 addendum point 4).
const maxErrorLine = 300

// diskFullText is what Docker prints when the image does not fit.
const diskFullText = "no space left on device"

// commandError is a failed docker call: the subcommand, its error (exit status) and the last
// non-empty line of its stderr, redacted as by lastLine.
type commandError struct {
	verb     string
	err      error
	line     string
	diskFull bool // stderr reported "no space left on device"
}

func newCommandError(verb string, err error, stderr []byte) *commandError {
	line := lastLine(stderr)
	if len(line) > maxErrorLine {
		line = strings.ToValidUTF8(line[:maxErrorLine], "")
	}
	return &commandError{verb: verb, err: err, line: line,
		diskFull: strings.Contains(strings.ToLower(string(stderr)), diskFullText)}
}

func (e *commandError) Error() string {
	if e.line == "" {
		return fmt.Sprintf("docker %s: %v", e.verb, e.err)
	}
	return fmt.Sprintf("docker %s: %v: %s", e.verb, e.err, e.line)
}

func (e *commandError) Unwrap() error { return e.err }

// errorLine returns the stderr line of a failed docker call ("" for another error).
func errorLine(err error) string {
	if ce := (*commandError)(nil); errors.As(err, &ce) {
		return ce.line
	}
	return ""
}

// diskFull reports whether a failed docker call ran out of disk space.
func diskFull(err error) bool {
	ce := (*commandError)(nil)
	return errors.As(err, &ce) && ce.diskFull
}

// maxLogLine caps the line lastLine returns.
const maxLogLine = 512

// assignment is KEY=value in command output; compose quotes lines of the env files in some
// errors, and their values are often secrets (CLAUDE.md rule 10).
var assignment = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_.-]*)=("[^"]*"?|'[^']*'?|[^\s,;]*)`)

// lastLine returns the last non-empty line of out, at most maxLogLine bytes, with the value of
// every KEY=value and of the secrets the logging package knows replaced by [REDACTED].
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return redactLine(lines[len(lines)-1])
}

// redactLine is one line of output as lastLine returns it.
func redactLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > maxLogLine {
		line = line[:maxLogLine]
	}
	line = assignment.ReplaceAllString(line, "${1}="+logging.Redacted)
	return logging.RedactString(strings.ToValidUTF8(line, "?"))
}

// redactLines returns the last n non-empty lines of out, each redacted as by lastLine.
func redactLines(out []byte, n int) []string {
	var lines []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, redactLine(line))
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
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
		Image  string
		Config struct {
			Image  string
			Labels map[string]string
		}
		State struct {
			Running  bool
			Status   string
			ExitCode int
			Health   *struct{ Status string }
		}
		RestartCount int
		HostConfig   struct {
			RestartPolicy struct {
				Name              string
				MaximumRetryCount int
			}
		}
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return Container{}, fmt.Errorf("inspect %s: %w", name, err)
	}
	if len(list) != 1 {
		return Container{}, fmt.Errorf("inspect %s: %d results", name, len(list))
	}
	c := list[0]
	res := Container{Image: c.Config.Image, ImageID: c.Image, Labels: c.Config.Labels, Running: c.State.Running,
		Status: c.State.Status, ExitCode: c.State.ExitCode, RestartCount: c.RestartCount,
		RestartPolicy: c.HostConfig.RestartPolicy.Name, MaxRetries: c.HostConfig.RestartPolicy.MaximumRetryCount}
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

// imageFormat prints the fields of Image, one image per line.
const imageFormat = "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.Digest}}"

func (d dockerCLI) Images(ctx context.Context, repository string) ([]Image, error) {
	if repository == "" {
		return nil, errors.New("image ls: no repository") // would list every image
	}
	out, err := d.run(ctx, "image", "ls", "--no-trunc", "--digests", "--format", imageFormat, repository)
	if err != nil {
		return nil, err
	}
	return parseImages(out), nil
}

// parseImages reads the output of docker image ls with imageFormat; a line without four fields
// is skipped.
func parseImages(out []byte) []Image {
	var list []Image
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) != 4 || f[0] == "" {
			continue
		}
		list = append(list, Image{ID: f[0], Repository: f[1], Tag: f[2], Digest: f[3]})
	}
	return list
}

func (d dockerCLI) RemoveImage(ctx context.Context, ref string) error {
	_, err := d.run(ctx, "image", "rm", ref)
	return err
}

func (d dockerCLI) Run(ctx context.Context, args ...string) error {
	_, err := d.run(ctx, append([]string{"run"}, args...)...)
	return err
}

func (d dockerCLI) RemoveContainer(ctx context.Context, name string, force bool) error {
	args := []string{"container", "rm"}
	if force {
		args = append(args, "--force")
	}
	_, err := d.run(ctx, append(args, name)...)
	return err
}

// Logs reads both streams of docker logs; the helper writes its log to the standard output.
func (d dockerCLI) Logs(ctx context.Context, name string, tail int) ([]string, error) {
	stdout, stderr, err := d.output(ctx, "logs", "--tail", strconv.Itoa(tail), name)
	if err != nil {
		return nil, err
	}
	return redactLines(append(stdout, stderr...), tail), nil
}
