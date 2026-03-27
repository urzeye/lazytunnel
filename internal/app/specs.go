package app

import (
	"fmt"

	"github.com/urzeye/lazytunnel/internal/adapters/kubernetes"
	"github.com/urzeye/lazytunnel/internal/adapters/ssh"
	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

func BuildProcessSpec(profile domain.Profile) (ltruntime.ProcessSpec, error) {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		return ssh.BuildProcessSpec(profile)
	case domain.TunnelTypeKubernetesPortForward:
		return kubernetes.BuildProcessSpec(profile)
	default:
		return ltruntime.ProcessSpec{}, fmt.Errorf("unsupported profile type %q", profile.Type)
	}
}
