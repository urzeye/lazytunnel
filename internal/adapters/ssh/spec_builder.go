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

	spec := ltruntime.ProcessSpec{
		Name:    profile.Name,
		Command: "ssh",
		Args: []string{
			"-N",
			"-o", "ExitOnForwardFailure=yes",
		},
		Restart: profile.Restart,
	}

	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		spec.Args = append(
			spec.Args,
			"-L", fmt.Sprintf("%d:%s:%d", profile.LocalPort, profile.SSH.RemoteHost, profile.SSH.RemotePort),
			profile.SSH.Host,
		)
	case domain.TunnelTypeSSHRemote:
		forward := fmt.Sprintf("%d:%s:%d", profile.SSHRemote.BindPort, profile.SSHRemote.TargetHost, profile.SSHRemote.TargetPort)
		if profile.SSHRemote.BindAddress != "" {
			forward = fmt.Sprintf("%s:%s", profile.SSHRemote.BindAddress, forward)
		}
		spec.Args = append(spec.Args, "-R", forward, profile.SSHRemote.Host)
	default:
		return ltruntime.ProcessSpec{}, fmt.Errorf(
			"profile %q is %q, want one of %q or %q",
			profile.Name,
			profile.Type,
			domain.TunnelTypeSSHLocal,
			domain.TunnelTypeSSHRemote,
		)
	}

	return spec, nil
}
