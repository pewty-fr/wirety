//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// Fake SSH: a TCP server on port 22 that greets every connection with an SSH
// banner, then echoes whatever the client sends — enough to tell "reachable"
// from "blocked", and to hold an interactive session open.

const fakeSSHBanner = "SSH-2.0-OpenSSH_9.6"

// startFakeSSH runs the fake SSH server in c.
func startFakeSSH(ctx context.Context, t *testing.T, c testcontainers.Container) {
	t.Helper()
	startBannerServer(ctx, t, c, 22, fakeSSHBanner)
}

// sshBanner opens a new connection from c to host:22 and returns the SSH
// banner, or "" when the connection is dropped or refused.
func sshBanner(ctx context.Context, t *testing.T, c testcontainers.Container, host string) string {
	t.Helper()
	return readBanner(ctx, t, c, host, 22, "SSH-2.0")
}

// startBannerServer runs a TCP server on port in c that greets every
// connection with banner, then echoes (busybox nc, one process per
// connection).
func startBannerServer(ctx context.Context, t *testing.T, c testcontainers.Container, port int, banner string) {
	t.Helper()
	mustExec(ctx, t, c, "sh", "-c", fmt.Sprintf("nc -lk -p %d -e sh -c 'echo %s; exec cat' >/dev/null 2>&1 &", port, banner))
}

// readBanner opens a new connection from c to host:port and returns what the
// server sent if it contains want, or "" when the connection is dropped or
// refused.
func readBanner(ctx context.Context, t *testing.T, c testcontainers.Container, host string, port int, want string) string {
	t.Helper()
	_, out, err := execInContainer(ctx, c, "sh", "-c", fmt.Sprintf("nc -w 3 %s %d </dev/null", host, port))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, want) {
		return strings.TrimSpace(out)
	}
	return ""
}

// sshSession is an open connection to a fake SSH server, typing a line every
// second like an interactive session; the server echoes each line back.
type sshSession struct {
	c    testcontainers.Container
	host string
	path string // log (echoes received) and pid files prefix
}

func openSSHSession(ctx context.Context, t *testing.T, c testcontainers.Container, host string) *sshSession {
	t.Helper()
	s := &sshSession{c: c, host: host, path: fmt.Sprintf("/tmp/ssh-session-%d", time.Now().UnixNano())}
	mustExec(ctx, t, c, "sh", "-c", fmt.Sprintf(
		"(while :; do echo ping; sleep 1; done) | nc %s 22 >%s.log 2>&1 & echo $! >%s.pid", host, s.path, s.path))
	return s
}

// echoes counts the lines the server echoed back so far.
func (s *sshSession) echoes(ctx context.Context, t *testing.T) int {
	t.Helper()
	_, out, err := execInContainer(ctx, s.c, "sh", "-c", "grep -c ping "+s.path+".log")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

// carriesData reports whether the session gets new echoes within window.
func (s *sshSession) carriesData(ctx context.Context, t *testing.T, window time.Duration) bool {
	t.Helper()
	before := s.echoes(ctx, t)
	for deadline := time.Now().Add(window); time.Now().Before(deadline); {
		time.Sleep(time.Second)
		if s.echoes(ctx, t) > before {
			return true
		}
	}
	return false
}

// close ends the session (kills the client).
func (s *sshSession) close(ctx context.Context) {
	_, _, _ = execInContainer(ctx, s.c, "sh", "-c", "kill $(cat "+s.path+".pid)")
}
