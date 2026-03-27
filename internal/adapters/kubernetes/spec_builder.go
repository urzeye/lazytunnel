package kubernetes

import (
	"fmt"

	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

func BuildProcessSpec(profile domain.Profile) (ltruntime.ProcessSpec, error) {
	if err := profile.Validate(); err != nil {
		return ltruntime.ProcessSpec{}, fmt.Errorf("validate profile: %w", err)
	}

	if profile.Type != domain.TunnelTypeKubernetesPortForward {
		return ltruntime.ProcessSpec{}, fmt.Errorf("profile %q is %q, want %q", profile.Name, profile.Type, domain.TunnelTypeKubernetesPortForward)
	}

	args := make([]string, 0, 8)
	if profile.Kubernetes.Context != "" {
		args = append(args, "--context", profile.Kubernetes.Context)
	}
	if profile.Kubernetes.Namespace != "" {
		args = append(args, "--namespace", profile.Kubernetes.Namespace)
	}

	args = append(
		args,
		"port-forward",
		resourceRef(profile.Kubernetes.ResourceType, profile.Kubernetes.Resource),
		fmt.Sprintf("%d:%d", profile.LocalPort, profile.Kubernetes.RemotePort),
		"--address", "127.0.0.1",
	)

	return ltruntime.ProcessSpec{
		Name:    profile.Name,
		Command: "kubectl",
		Args:    args,
		Restart: profile.Restart,
	}, nil
}

func resourceRef(resourceType, resource string) string {
	return fmt.Sprintf("%s/%s", resourceType, resource)
}
