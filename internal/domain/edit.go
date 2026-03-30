package domain

import "strings"

// PrepareProfileForType keeps common profile metadata while ensuring the
// type-specific section exists with sensible editing defaults.
func PrepareProfileForType(profile Profile, tunnelType TunnelType) Profile {
	profile.Type = tunnelType

	switch tunnelType {
	case TunnelTypeSSHRemote:
		host := firstNonEmpty(
			valueSSHRemoteHost(profile),
			valueSSHHost(profile),
			valueSSHDynamicHost(profile),
		)
		bindAddress := firstNonEmpty(
			valueSSHRemoteBindAddress(profile),
			valueSSHDynamicBindAddress(profile),
		)
		bindPort := firstPositive(
			valueSSHRemoteBindPort(profile),
			profile.LocalPort,
			9000,
		)
		targetHost := firstNonEmpty(
			valueSSHRemoteTargetHost(profile),
			valueSSHRemoteHostTarget(profile),
			"127.0.0.1",
		)
		targetPort := firstPositive(
			valueSSHRemoteTargetPort(profile),
			valueSSHRemoteHostPort(profile),
			valueKubernetesRemotePort(profile),
			8080,
		)

		profile.LocalPort = bindPort
		profile.SSHRemote = &SSHRemote{
			Host:        host,
			BindAddress: bindAddress,
			BindPort:    bindPort,
			TargetHost:  targetHost,
			TargetPort:  targetPort,
		}
		profile.SSH = nil
		profile.SSHDynamic = nil
		profile.Kubernetes = nil

	case TunnelTypeSSHDynamic:
		host := firstNonEmpty(
			valueSSHDynamicHost(profile),
			valueSSHHost(profile),
			valueSSHRemoteHost(profile),
		)
		bindAddress := firstNonEmpty(
			valueSSHDynamicBindAddress(profile),
			valueSSHRemoteBindAddress(profile),
			"127.0.0.1",
		)
		localPort := firstPositive(profile.LocalPort, 1080)

		profile.LocalPort = localPort
		profile.SSHDynamic = &SSHDynamic{
			Host:        host,
			BindAddress: bindAddress,
		}
		profile.SSH = nil
		profile.SSHRemote = nil
		profile.Kubernetes = nil

	case TunnelTypeKubernetesPortForward:
		localPort := firstPositive(profile.LocalPort, 8080)
		resourceType := firstNonEmpty(valueKubernetesResourceType(profile), "service")
		remotePort := firstPositive(
			valueKubernetesRemotePort(profile),
			valueSSHRemoteHostPort(profile),
			valueSSHRemoteTargetPort(profile),
			80,
		)

		profile.LocalPort = localPort
		profile.Kubernetes = &Kubernetes{
			Context:      valueKubernetesContext(profile),
			Namespace:    valueKubernetesNamespace(profile),
			ResourceType: resourceType,
			Resource:     valueKubernetesResource(profile),
			RemotePort:   remotePort,
		}
		profile.SSH = nil
		profile.SSHRemote = nil
		profile.SSHDynamic = nil

	case TunnelTypeSSHLocal:
		fallthrough
	default:
		host := firstNonEmpty(
			valueSSHHost(profile),
			valueSSHDynamicHost(profile),
			valueSSHRemoteHost(profile),
		)
		remoteHost := firstNonEmpty(
			valueSSHRemoteHostTarget(profile),
			valueSSHRemoteTargetHost(profile),
			"127.0.0.1",
		)
		remotePort := firstPositive(
			valueSSHRemoteHostPort(profile),
			valueSSHRemoteTargetPort(profile),
			valueKubernetesRemotePort(profile),
			5432,
		)
		localPort := firstPositive(profile.LocalPort, 15432)

		profile.LocalPort = localPort
		profile.SSH = &SSHLocal{
			Host:       host,
			RemoteHost: remoteHost,
			RemotePort: remotePort,
		}
		profile.SSHRemote = nil
		profile.SSHDynamic = nil
		profile.Kubernetes = nil
	}

	return profile
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func valueSSHHost(profile Profile) string {
	if profile.SSH == nil {
		return ""
	}
	return profile.SSH.Host
}

func valueSSHRemoteHost(profile Profile) string {
	if profile.SSHRemote == nil {
		return ""
	}
	return profile.SSHRemote.Host
}

func valueSSHRemoteBindAddress(profile Profile) string {
	if profile.SSHRemote == nil {
		return ""
	}
	return profile.SSHRemote.BindAddress
}

func valueSSHRemoteBindPort(profile Profile) int {
	if profile.SSHRemote == nil {
		return 0
	}
	return profile.SSHRemote.BindPort
}

func valueSSHRemoteTargetHost(profile Profile) string {
	if profile.SSHRemote == nil {
		return ""
	}
	return profile.SSHRemote.TargetHost
}

func valueSSHRemoteTargetPort(profile Profile) int {
	if profile.SSHRemote == nil {
		return 0
	}
	return profile.SSHRemote.TargetPort
}

func valueSSHRemoteHostTarget(profile Profile) string {
	if profile.SSH == nil {
		return ""
	}
	return profile.SSH.RemoteHost
}

func valueSSHRemoteHostPort(profile Profile) int {
	if profile.SSH == nil {
		return 0
	}
	return profile.SSH.RemotePort
}

func valueSSHDynamicHost(profile Profile) string {
	if profile.SSHDynamic == nil {
		return ""
	}
	return profile.SSHDynamic.Host
}

func valueSSHDynamicBindAddress(profile Profile) string {
	if profile.SSHDynamic == nil {
		return ""
	}
	return profile.SSHDynamic.BindAddress
}

func valueKubernetesContext(profile Profile) string {
	if profile.Kubernetes == nil {
		return ""
	}
	return profile.Kubernetes.Context
}

func valueKubernetesNamespace(profile Profile) string {
	if profile.Kubernetes == nil {
		return ""
	}
	return profile.Kubernetes.Namespace
}

func valueKubernetesResourceType(profile Profile) string {
	if profile.Kubernetes == nil {
		return ""
	}
	return profile.Kubernetes.ResourceType
}

func valueKubernetesResource(profile Profile) string {
	if profile.Kubernetes == nil {
		return ""
	}
	return profile.Kubernetes.Resource
}

func valueKubernetesRemotePort(profile Profile) int {
	if profile.Kubernetes == nil {
		return 0
	}
	return profile.Kubernetes.RemotePort
}
