package ssh

import (
	"fmt"

	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

func BuildProcessSpec(profile domain.Profile) (ltruntime.ProcessSpec, error) {
	if err := profile.Validate(); err != nil {
		return ltruntime.ProcessSpec{}, fmt.Errorf("validate profile: %w", err)
	}

	if profile.Type != domain.TunnelTypeSSHLocal {
		return ltruntime.ProcessSpec{}, fmt.Errorf("profile %q is %q, want %q", profile.Name, profile.Type, domain.TunnelTypeSSHLocal)
	}

	return ltruntime.ProcessSpec{
		Name:    profile.Name,
		Command: "ssh",
		Args: []string{
			"-N",
			"-o", "ExitOnForwardFailure=yes",
			"-L", fmt.Sprintf("%d:%s:%d", profile.LocalPort, profile.SSH.RemoteHost, profile.SSH.RemotePort),
			profile.SSH.Host,
		},
		Restart: profile.Restart,
	}, nil
}
