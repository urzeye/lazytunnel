package domain

import "time"

type TunnelStatus string

const (
	TunnelStatusStopped    TunnelStatus = "stopped"
	TunnelStatusStarting   TunnelStatus = "starting"
	TunnelStatusRunning    TunnelStatus = "running"
	TunnelStatusRestarting TunnelStatus = "restarting"
	TunnelStatusExited     TunnelStatus = "exited"
	TunnelStatusFailed     TunnelStatus = "failed"
)

type LogSource string

const (
	LogSourceStdout LogSource = "stdout"
	LogSourceStderr LogSource = "stderr"
	LogSourceSystem LogSource = "system"
)

type LogEntry struct {
	Timestamp time.Time
	Source    LogSource
	Message   string
}

type RuntimeState struct {
	ProfileName  string
	Status       TunnelStatus
	PID          int
	StartedAt    *time.Time
	ExitedAt     *time.Time
	RestartCount int
	LastExitCode int
	LastError    string
	ExitReason   string
	RecentLogs   []LogEntry
}

func (s RuntimeState) Clone() RuntimeState {
	cloned := s
	cloned.RecentLogs = append([]LogEntry(nil), s.RecentLogs...)
	return cloned
}
