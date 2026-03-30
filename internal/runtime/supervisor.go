package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/urzeye/lazytunnel/internal/domain"
)

const (
	defaultInitialBackoff = 2 * time.Second
	defaultMaxBackoff     = 30 * time.Second
	defaultMaxRecentLogs  = 200
)

type ProcessSpec struct {
	Name    string
	Command string
	Args    []string
	Dir     string
	Env     []string
	Restart domain.RestartPolicy
}

type EventType string

const (
	EventTypeStateChanged EventType = "state_changed"
	EventTypeLog          EventType = "log"
)

type Event struct {
	Type  EventType
	Name  string
	State domain.RuntimeState
	Log   *domain.LogEntry
}

type Process interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
	PID() int
	ExitCode() int
}

type ProcessFactory interface {
	Create(ctx context.Context, spec ProcessSpec) (Process, error)
}

type SupervisorOption func(*Supervisor)

func WithNow(now func() time.Time) SupervisorOption {
	return func(s *Supervisor) {
		s.now = now
	}
}

func WithSleep(sleep func(time.Duration)) SupervisorOption {
	return func(s *Supervisor) {
		s.sleep = sleep
	}
}

func WithMaxRecentLogs(limit int) SupervisorOption {
	return func(s *Supervisor) {
		if limit > 0 {
			s.maxRecentLogs = limit
		}
	}
}

type Supervisor struct {
	mu            sync.RWMutex
	factory       ProcessFactory
	now           func() time.Time
	sleep         func(time.Duration)
	maxRecentLogs int
	processes     map[string]*managedProcess
	subscribers   map[int]chan Event
	nextSubID     int
}

type managedProcess struct {
	spec   ProcessSpec
	cancel context.CancelFunc
	done   chan struct{}
	state  domain.RuntimeState
}

func NewSupervisor(factory ProcessFactory, opts ...SupervisorOption) *Supervisor {
	supervisor := &Supervisor{
		factory:       factory,
		now:           time.Now,
		sleep:         time.Sleep,
		maxRecentLogs: defaultMaxRecentLogs,
		processes:     make(map[string]*managedProcess),
		subscribers:   make(map[int]chan Event),
	}

	for _, opt := range opts {
		opt(supervisor)
	}

	return supervisor
}

func (s *Supervisor) Start(spec ProcessSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, exists := s.processes[spec.Name]; exists && !isClosed(existing.done) {
		return fmt.Errorf("process %q is already running", spec.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	process := &managedProcess{
		spec:   spec,
		cancel: cancel,
		done:   make(chan struct{}),
		state: domain.RuntimeState{
			ProfileName: spec.Name,
			Status:      domain.TunnelStatusStarting,
		},
	}

	s.processes[spec.Name] = process
	go s.run(ctx, process)

	return nil
}

func (s *Supervisor) Stop(name string) error {
	s.mu.RLock()
	process, exists := s.processes[name]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("process %q not found", name)
	}

	process.cancel()
	<-process.done
	return nil
}

func (s *Supervisor) ClearLogs(name string) error {
	s.mu.Lock()
	process, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("process %q not found", name)
	}

	process.state.RecentLogs = nil
	stateSnapshot := process.state.Clone()
	subscribers := s.snapshotSubscribersLocked()
	s.mu.Unlock()

	s.publish(subscribers, Event{
		Type:  EventTypeStateChanged,
		Name:  name,
		State: stateSnapshot,
	})
	return nil
}

func (s *Supervisor) Snapshot(name string) (domain.RuntimeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	process, exists := s.processes[name]
	if !exists {
		return domain.RuntimeState{}, false
	}

	return process.state.Clone(), true
}

func (s *Supervisor) ListStates() []domain.RuntimeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]domain.RuntimeState, 0, len(s.processes))
	for _, process := range s.processes {
		states = append(states, process.state.Clone())
	}

	slices.SortFunc(states, func(a, b domain.RuntimeState) int {
		switch {
		case a.ProfileName < b.ProfileName:
			return -1
		case a.ProfileName > b.ProfileName:
			return 1
		default:
			return 0
		}
	})

	return states
}

func (s *Supervisor) Subscribe(buffer int) (int, <-chan Event) {
	if buffer <= 0 {
		buffer = 32
	}

	ch := make(chan Event, buffer)

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextSubID
	s.nextSubID++
	s.subscribers[id] = ch

	return id, ch
}

func (s *Supervisor) Unsubscribe(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.subscribers[id]; exists {
		delete(s.subscribers, id)
		close(ch)
	}
}

func (s *Supervisor) run(ctx context.Context, process *managedProcess) {
	defer close(process.done)

	restartCount := 0

	for {
		if ctx.Err() != nil {
			s.markStopped(process.spec.Name, 0, "stopped by user")
			return
		}

		status := domain.TunnelStatusStarting
		if restartCount > 0 {
			status = domain.TunnelStatusRestarting
		}

		s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
			state.Status = status
			state.PID = 0
			state.StartedAt = nil
			state.ExitedAt = nil
			state.ExitReason = ""
			state.LastError = ""
			state.LastExitCode = 0
			state.RestartCount = restartCount
		})
		s.appendSystemLog(process.spec.Name, fmt.Sprintf("starting command: %s", process.spec.DisplayCommand()))

		runtimeProcess, err := s.factory.Create(ctx, process.spec)
		if err != nil {
			if s.handleLaunchError(ctx, process.spec, restartCount, fmt.Errorf("create process: %w", err)) {
				restartCount++
				continue
			}
			return
		}

		stdout, err := runtimeProcess.StdoutPipe()
		if err != nil {
			if s.handleLaunchError(ctx, process.spec, restartCount, fmt.Errorf("stdout pipe: %w", err)) {
				restartCount++
				continue
			}
			return
		}

		stderr, err := runtimeProcess.StderrPipe()
		if err != nil {
			if s.handleLaunchError(ctx, process.spec, restartCount, fmt.Errorf("stderr pipe: %w", err)) {
				restartCount++
				continue
			}
			return
		}

		if err := runtimeProcess.Start(); err != nil {
			if s.handleLaunchError(ctx, process.spec, restartCount, fmt.Errorf("start process: %w", err)) {
				restartCount++
				continue
			}
			return
		}

		startedAt := s.now()
		s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
			state.Status = domain.TunnelStatusRunning
			state.PID = runtimeProcess.PID()
			state.StartedAt = &startedAt
			state.ExitedAt = nil
			state.ExitReason = ""
			state.LastError = ""
			state.LastExitCode = 0
			state.RestartCount = restartCount
		})
		s.appendSystemLog(process.spec.Name, fmt.Sprintf("process started with pid %d", runtimeProcess.PID()))

		var readers sync.WaitGroup
		readers.Add(2)
		go s.captureLogs(process.spec.Name, domain.LogSourceStdout, stdout, &readers)
		go s.captureLogs(process.spec.Name, domain.LogSourceStderr, stderr, &readers)

		waitErr := runtimeProcess.Wait()
		readers.Wait()

		if ctx.Err() != nil {
			exitedAt := s.now()
			s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
				state.Status = domain.TunnelStatusStopped
				state.PID = 0
				state.ExitedAt = &exitedAt
				state.ExitReason = "stopped by user"
				state.LastError = ""
				state.LastExitCode = runtimeProcess.ExitCode()
			})
			s.appendSystemLog(process.spec.Name, "process stopped")
			return
		}

		exitedAt := s.now()
		exitCode := runtimeProcess.ExitCode()
		exitReason := "process exited"
		if waitErr != nil {
			exitReason = waitErr.Error()
		}

		s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
			state.PID = 0
			state.ExitedAt = &exitedAt
			state.LastExitCode = exitCode
			state.ExitReason = exitReason
		})
		s.appendSystemLog(process.spec.Name, fmt.Sprintf("process exited with code %d", exitCode))

		if s.shouldRestart(process.spec.Restart, restartCount) {
			backoff := restartBackoff(process.spec.Restart, restartCount)
			s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
				state.Status = domain.TunnelStatusRestarting
				state.RestartCount = restartCount + 1
			})
			s.appendSystemLog(process.spec.Name, fmt.Sprintf("restarting in %s", backoff))
			if !s.waitBackoff(ctx, backoff) {
				s.markStopped(process.spec.Name, exitCode, "stopped by user")
				return
			}
			restartCount++
			continue
		}

		finalStatus := domain.TunnelStatusExited
		lastError := ""
		if waitErr != nil {
			finalStatus = domain.TunnelStatusFailed
			lastError = waitErr.Error()
		}

		s.updateState(process.spec.Name, func(state *domain.RuntimeState) {
			state.Status = finalStatus
			state.LastError = lastError
			state.RestartCount = restartCount
		})
		return
	}
}

func (s *Supervisor) handleLaunchError(ctx context.Context, spec ProcessSpec, restartCount int, err error) bool {
	if ctx.Err() != nil {
		s.markStopped(spec.Name, 0, "stopped by user")
		return false
	}

	s.updateState(spec.Name, func(state *domain.RuntimeState) {
		state.Status = domain.TunnelStatusFailed
		state.LastError = err.Error()
		state.ExitReason = err.Error()
		state.PID = 0
		state.StartedAt = nil
		now := s.now()
		state.ExitedAt = &now
	})
	s.appendSystemLog(spec.Name, err.Error())

	if !s.shouldRestart(spec.Restart, restartCount) {
		return false
	}

	backoff := restartBackoff(spec.Restart, restartCount)
	s.updateState(spec.Name, func(state *domain.RuntimeState) {
		state.Status = domain.TunnelStatusRestarting
		state.RestartCount = restartCount + 1
	})
	s.appendSystemLog(spec.Name, fmt.Sprintf("retrying after launch failure in %s", backoff))
	if !s.waitBackoff(ctx, backoff) {
		s.markStopped(spec.Name, 0, "stopped by user")
		return false
	}

	return true
}

func (s *Supervisor) shouldRestart(policy domain.RestartPolicy, restartCount int) bool {
	if !policy.Enabled {
		return false
	}

	if policy.MaxRetries == 0 {
		return true
	}

	return restartCount < policy.MaxRetries
}

func (s *Supervisor) captureLogs(name string, source domain.LogSource, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		entry := domain.LogEntry{
			Timestamp: s.now(),
			Source:    source,
			Message:   line,
		}
		s.appendLog(name, entry)
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		s.appendSystemLog(name, fmt.Sprintf("log reader error: %v", err))
	}
}

func (s *Supervisor) appendSystemLog(name, message string) {
	s.appendLog(name, domain.LogEntry{
		Timestamp: s.now(),
		Source:    domain.LogSourceSystem,
		Message:   message,
	})
}

func (s *Supervisor) appendLog(name string, entry domain.LogEntry) {
	s.mu.Lock()
	process, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return
	}

	process.state.RecentLogs = append(process.state.RecentLogs, entry)
	if len(process.state.RecentLogs) > s.maxRecentLogs {
		process.state.RecentLogs = append([]domain.LogEntry(nil), process.state.RecentLogs[len(process.state.RecentLogs)-s.maxRecentLogs:]...)
	}

	stateSnapshot := process.state.Clone()
	subscribers := s.snapshotSubscribersLocked()
	s.mu.Unlock()

	logCopy := entry
	s.publish(subscribers, Event{
		Type:  EventTypeLog,
		Name:  name,
		State: stateSnapshot,
		Log:   &logCopy,
	})
}

func (s *Supervisor) updateState(name string, mutate func(*domain.RuntimeState)) {
	s.mu.Lock()
	process, exists := s.processes[name]
	if !exists {
		s.mu.Unlock()
		return
	}

	mutate(&process.state)
	stateSnapshot := process.state.Clone()
	subscribers := s.snapshotSubscribersLocked()
	s.mu.Unlock()

	s.publish(subscribers, Event{
		Type:  EventTypeStateChanged,
		Name:  name,
		State: stateSnapshot,
	})
}

func (s *Supervisor) snapshotSubscribersLocked() []chan Event {
	subscribers := make([]chan Event, 0, len(s.subscribers))
	for _, ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}

	return subscribers
}

func (s *Supervisor) publish(subscribers []chan Event, event Event) {
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func restartBackoff(policy domain.RestartPolicy, restartCount int) time.Duration {
	initial := defaultInitialBackoff
	if policy.InitialBackoff != "" {
		if parsed, err := time.ParseDuration(policy.InitialBackoff); err == nil && parsed > 0 {
			initial = parsed
		}
	}

	maxBackoff := defaultMaxBackoff
	if policy.MaxBackoff != "" {
		if parsed, err := time.ParseDuration(policy.MaxBackoff); err == nil && parsed > 0 {
			maxBackoff = parsed
		}
	}

	backoff := initial
	for i := 0; i < restartCount; i++ {
		backoff *= 2
		if backoff >= maxBackoff {
			return maxBackoff
		}
	}

	if backoff > maxBackoff {
		return maxBackoff
	}

	return backoff
}

func (s ProcessSpec) Validate() error {
	switch {
	case strings.TrimSpace(s.Name) == "":
		return errors.New("process name is required")
	case strings.TrimSpace(s.Command) == "":
		return errors.New("command is required")
	default:
		return nil
	}
}

func (s ProcessSpec) DisplayCommand() string {
	if len(s.Args) == 0 {
		return s.Command
	}

	return s.Command + " " + strings.Join(s.Args, " ")
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func (s *Supervisor) waitBackoff(ctx context.Context, duration time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.sleep(duration)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return false
	case <-done:
		return true
	}
}

func (s *Supervisor) markStopped(name string, exitCode int, reason string) {
	now := s.now()
	s.updateState(name, func(state *domain.RuntimeState) {
		state.Status = domain.TunnelStatusStopped
		state.PID = 0
		state.ExitedAt = &now
		state.ExitReason = reason
		state.LastError = ""
		state.LastExitCode = exitCode
	})
	s.appendSystemLog(name, "process stopped")
}

type ExecProcessFactory struct{}

func (ExecProcessFactory) Create(ctx context.Context, spec ProcessSpec) (Process, error) {
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}

	return &execProcess{cmd: cmd}, nil
}

type execProcess struct {
	cmd *exec.Cmd
}

func (p *execProcess) StdoutPipe() (io.ReadCloser, error) {
	return p.cmd.StdoutPipe()
}

func (p *execProcess) StderrPipe() (io.ReadCloser, error) {
	return p.cmd.StderrPipe()
}

func (p *execProcess) Start() error {
	return p.cmd.Start()
}

func (p *execProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *execProcess) PID() int {
	if p.cmd.Process == nil {
		return 0
	}

	return p.cmd.Process.Pid
}

func (p *execProcess) ExitCode() int {
	if p.cmd.ProcessState == nil {
		return 0
	}

	return p.cmd.ProcessState.ExitCode()
}
