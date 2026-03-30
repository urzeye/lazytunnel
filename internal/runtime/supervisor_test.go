package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestSupervisorCapturesLogsAndExitState(t *testing.T) {
	t.Parallel()

	clock := newFakeClock(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	supervisor := NewSupervisor(
		&fakeProcessFactory{
			processes: []fakeProcessPlan{
				{
					pid:        1001,
					stdout:     []string{"forward ready"},
					stderr:     []string{"warning: tunnel warmup"},
					waitErr:    nil,
					exitCode:   0,
					waitForCtx: false,
				},
			},
		},
		WithNow(clock.Now),
		WithSleep(clock.Sleep),
		WithMaxRecentLogs(16),
	)

	if err := supervisor.Start(ProcessSpec{
		Name:    "prod-db",
		Command: "fake-ssh",
		Args:    []string{"-L", "5432:db.internal:5432", "bastion"},
	}); err != nil {
		t.Fatalf("start process: %v", err)
	}

	state := waitForStatus(t, supervisor, "prod-db", domain.TunnelStatusExited)
	if state.LastExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", state.LastExitCode)
	}

	if state.PID != 0 {
		t.Fatalf("expected pid to be cleared after exit, got %d", state.PID)
	}

	logs := collectMessages(state.RecentLogs)
	if !strings.Contains(logs, "forward ready") {
		t.Fatalf("expected stdout log, got %q", logs)
	}

	if !strings.Contains(logs, "warning: tunnel warmup") {
		t.Fatalf("expected stderr log, got %q", logs)
	}
}

func TestSupervisorRestartsWithBackoff(t *testing.T) {
	t.Parallel()

	clock := newFakeClock(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	supervisor := NewSupervisor(
		&fakeProcessFactory{
			processes: []fakeProcessPlan{
				{
					pid:      1001,
					stdout:   []string{"first launch"},
					waitErr:  errors.New("lost connection"),
					exitCode: 255,
				},
				{
					pid:      1002,
					stdout:   []string{"second launch"},
					waitErr:  nil,
					exitCode: 0,
				},
			},
		},
		WithNow(clock.Now),
		WithSleep(clock.Sleep),
	)

	err := supervisor.Start(ProcessSpec{
		Name:    "api-debug",
		Command: "fake-kubectl",
		Args:    []string{"port-forward", "svc/api", "8080:80"},
		Restart: domain.RestartPolicy{
			Enabled:        true,
			MaxRetries:     1,
			InitialBackoff: "3s",
			MaxBackoff:     "30s",
		},
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}

	state := waitForStatus(t, supervisor, "api-debug", domain.TunnelStatusExited)
	if state.RestartCount != 1 {
		t.Fatalf("expected restart count 1, got %d", state.RestartCount)
	}

	if got := clock.Sleeps(); len(got) != 1 || got[0] != 3*time.Second {
		t.Fatalf("expected one 3s backoff sleep, got %v", got)
	}

	logs := collectMessages(state.RecentLogs)
	if !strings.Contains(logs, "restarting in 3s") {
		t.Fatalf("expected restart log, got %q", logs)
	}
}

func TestSupervisorClearLogsRemovesRecentLogHistory(t *testing.T) {
	t.Parallel()

	clock := newFakeClock(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	supervisor := NewSupervisor(
		&fakeProcessFactory{
			processes: []fakeProcessPlan{
				{
					pid:      1001,
					stdout:   []string{"forward ready"},
					waitErr:  nil,
					exitCode: 0,
				},
			},
		},
		WithNow(clock.Now),
		WithSleep(clock.Sleep),
	)

	if err := supervisor.Start(ProcessSpec{
		Name:    "prod-db",
		Command: "fake-ssh",
		Args:    []string{"-L", "5432:db.internal:5432", "bastion"},
	}); err != nil {
		t.Fatalf("start process: %v", err)
	}

	state := waitForStatus(t, supervisor, "prod-db", domain.TunnelStatusExited)
	if len(state.RecentLogs) == 0 {
		t.Fatal("expected logs before clear")
	}

	if err := supervisor.ClearLogs("prod-db"); err != nil {
		t.Fatalf("clear logs: %v", err)
	}

	state, ok := supervisor.Snapshot("prod-db")
	if !ok {
		t.Fatal("expected cleared state snapshot")
	}
	if len(state.RecentLogs) != 0 {
		t.Fatalf("expected logs to be cleared, got %#v", state.RecentLogs)
	}
}

func TestSupervisorStopCancelsRunningProcess(t *testing.T) {
	t.Parallel()

	clock := newFakeClock(time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC))
	supervisor := NewSupervisor(
		&fakeProcessFactory{
			processes: []fakeProcessPlan{
				{
					pid:        1003,
					stdout:     []string{"ready"},
					waitErr:    context.Canceled,
					exitCode:   -1,
					waitForCtx: true,
				},
			},
		},
		WithNow(clock.Now),
		WithSleep(clock.Sleep),
	)

	if err := supervisor.Start(ProcessSpec{
		Name:    "long-running",
		Command: "fake-ssh",
		Args:    []string{"-L", "6379:redis.internal:6379", "bastion"},
		Restart: domain.RestartPolicy{
			Enabled:        true,
			InitialBackoff: "1s",
		},
	}); err != nil {
		t.Fatalf("start process: %v", err)
	}

	waitForStatus(t, supervisor, "long-running", domain.TunnelStatusRunning)

	if err := supervisor.Stop("long-running"); err != nil {
		t.Fatalf("stop process: %v", err)
	}

	state := waitForStatus(t, supervisor, "long-running", domain.TunnelStatusStopped)
	if state.RestartCount != 0 {
		t.Fatalf("expected no restart after manual stop, got %d", state.RestartCount)
	}

	if got := clock.Sleeps(); len(got) != 0 {
		t.Fatalf("expected no backoff sleep on manual stop, got %v", got)
	}
}

func waitForStatus(t *testing.T, supervisor *Supervisor, name string, want domain.TunnelStatus) domain.RuntimeState {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, ok := supervisor.Snapshot(name)
		if ok && state.Status == want {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}

	state, _ := supervisor.Snapshot(name)
	t.Fatalf("timed out waiting for %s, last state: %+v", want, state)
	return domain.RuntimeState{}
}

func collectMessages(entries []domain.LogEntry) string {
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s:%s", entry.Source, entry.Message))
	}

	return strings.Join(parts, "\n")
}

type fakeProcessFactory struct {
	mu        sync.Mutex
	processes []fakeProcessPlan
	index     int
}

func (f *fakeProcessFactory) Create(ctx context.Context, spec ProcessSpec) (Process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.index >= len(f.processes) {
		return nil, fmt.Errorf("unexpected process creation for %q", spec.Name)
	}

	plan := f.processes[f.index]
	f.index++

	return newFakeProcess(ctx, plan), nil
}

type fakeProcessPlan struct {
	pid        int
	stdout     []string
	stderr     []string
	waitErr    error
	exitCode   int
	waitForCtx bool
}

type fakeProcess struct {
	ctx           context.Context
	plan          fakeProcessPlan
	stdoutReader  *io.PipeReader
	stdoutWriter  *io.PipeWriter
	stderrReader  *io.PipeReader
	stderrWriter  *io.PipeWriter
	finishOnce    sync.Once
	finished      chan struct{}
	startFinished bool
}

func newFakeProcess(ctx context.Context, plan fakeProcessPlan) *fakeProcess {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	return &fakeProcess{
		ctx:          ctx,
		plan:         plan,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
		stderrReader: stderrReader,
		stderrWriter: stderrWriter,
		finished:     make(chan struct{}),
	}
}

func (p *fakeProcess) StdoutPipe() (io.ReadCloser, error) {
	return p.stdoutReader, nil
}

func (p *fakeProcess) StderrPipe() (io.ReadCloser, error) {
	return p.stderrReader, nil
}

func (p *fakeProcess) Start() error {
	if p.startFinished {
		return errors.New("process already started")
	}

	p.startFinished = true
	go func() {
		for _, line := range p.plan.stdout {
			_, _ = fmt.Fprintln(p.stdoutWriter, line)
		}
		for _, line := range p.plan.stderr {
			_, _ = fmt.Fprintln(p.stderrWriter, line)
		}

		if !p.plan.waitForCtx {
			p.finish()
		}
	}()

	if p.plan.waitForCtx {
		go func() {
			<-p.ctx.Done()
			p.finish()
		}()
	}

	return nil
}

func (p *fakeProcess) Wait() error {
	<-p.finished
	return p.plan.waitErr
}

func (p *fakeProcess) PID() int {
	return p.plan.pid
}

func (p *fakeProcess) ExitCode() int {
	return p.plan.exitCode
}

func (p *fakeProcess) finish() {
	p.finishOnce.Do(func() {
		_ = p.stdoutWriter.Close()
		_ = p.stderrWriter.Close()
		close(p.finished)
	})
}

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.now
	c.now = c.now.Add(time.Second)
	return current
}

func (c *fakeClock) Sleep(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
}

func (c *fakeClock) Sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]time.Duration(nil), c.sleeps...)
}
