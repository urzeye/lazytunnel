package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/urzeye/lazytunnel/internal/domain"
	profileimport "github.com/urzeye/lazytunnel/internal/profileimport"
)

const defaultSystemProfileProbeTTL = 5 * time.Second

type ProfileProbeResult struct {
	Problems []string
	Warnings []string
}

type ProfileProbeChecker interface {
	CheckProfile(profile domain.Profile, force bool) ProfileProbeResult
}

type commandOutputRunner interface {
	CombinedOutput(ctx context.Context, name string, args []string) ([]byte, error)
}

type execCommandOutputRunner struct{}

func (execCommandOutputRunner) CombinedOutput(ctx context.Context, name string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type noopProfileProbeChecker struct{}

func (noopProfileProbeChecker) CheckProfile(domain.Profile, bool) ProfileProbeResult {
	return ProfileProbeResult{}
}

type cachedProfileProbeResult struct {
	result    ProfileProbeResult
	checkedAt time.Time
}

type systemProfileProbeChecker struct {
	mu     sync.Mutex
	now    func() time.Time
	ttl    time.Duration
	runner commandOutputRunner
	cache  map[string]cachedProfileProbeResult
}

func newSystemProfileProbeChecker() *systemProfileProbeChecker {
	return &systemProfileProbeChecker{
		now:    time.Now,
		ttl:    defaultSystemProfileProbeTTL,
		runner: execCommandOutputRunner{},
		cache:  make(map[string]cachedProfileProbeResult),
	}
}

func (c *systemProfileProbeChecker) CheckProfile(profile domain.Profile, force bool) ProfileProbeResult {
	cacheKey := profileProbeCacheKey(profile)
	if !force {
		c.mu.Lock()
		cached, exists := c.cache[cacheKey]
		c.mu.Unlock()
		if exists && c.now().Sub(cached.checkedAt) < c.ttl {
			return cached.result
		}
	}

	result := c.computeProfile(profile)

	c.mu.Lock()
	c.cache[cacheKey] = cachedProfileProbeResult{
		result:    result,
		checkedAt: c.now(),
	}
	c.mu.Unlock()

	return result
}

func (c *systemProfileProbeChecker) computeProfile(profile domain.Profile) ProfileProbeResult {
	result := ProfileProbeResult{}

	switch profile.Type {
	case domain.TunnelTypeSSHLocal, domain.TunnelTypeSSHRemote, domain.TunnelTypeSSHDynamic:
		host := sshHostForProfile(profile)
		result.Warnings = append(result.Warnings, c.checkSSHHostAlias(host)...)
	case domain.TunnelTypeKubernetesPortForward:
		result = c.checkKubernetesTarget(profile)
	}

	return result
}

func (c *systemProfileProbeChecker) checkSSHHostAlias(host string) []string {
	host = strings.TrimSpace(host)
	if host == "" || looksLikeDirectSSHHost(host) {
		return nil
	}

	var warnings []string
	inspection, output, err := c.inspectSSHHost(host)
	resolvedBySSH := false
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("could not verify SSH host alias %q with ssh -G: %s", host, commandOutputSummary(output, err)))
	} else if strings.TrimSpace(inspection.HostName) != "" && !strings.EqualFold(strings.TrimSpace(inspection.HostName), host) {
		resolvedBySSH = true
	}

	entry, sourcePath, exists, err := profileimport.LookupSSHConfigHost("", host)
	if err != nil {
		if len(warnings) > 0 {
			return warnings
		}
		return []string{
			fmt.Sprintf(
				"could not verify SSH host alias %q from %s: %v",
				host,
				profileimport.DefaultSSHConfigPath(),
				err,
			),
		}
	}

	if exists {
		warnings = append(warnings, checkSSHIdentityFiles(host, entry.IdentityFiles)...)
		return warnings
	}

	if resolvedBySSH {
		return warnings
	}

	warnings = append(warnings,
		fmt.Sprintf(
			"SSH host alias %q was not found in %s; ssh will treat it as a raw hostname",
			host,
			sourcePath,
		),
	)
	return warnings
}

func (c *systemProfileProbeChecker) checkKubernetesTarget(profile domain.Profile) ProfileProbeResult {
	if profile.Kubernetes == nil {
		return ProfileProbeResult{}
	}

	result := ProfileProbeResult{}
	resourceName := strings.TrimSpace(profile.Kubernetes.Resource)
	if strings.EqualFold(resourceName, "change-me") {
		result.Warnings = append(result.Warnings, `resource "change-me" is still a placeholder`)
	}

	cfg, importResult, err := profileimport.ImportKubeContexts(domain.DefaultConfig(), "", false)
	if err != nil {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"could not verify Kubernetes context information from %s: %v",
				profileimport.DefaultKubeconfigPath(),
				err,
			),
		)
		return result
	}

	contextName := strings.TrimSpace(profile.Kubernetes.Context)
	if contextName != "" && !hasNamedProfile(cfg.Profiles, contextName) {
		result.Problems = append(
			result.Problems,
			fmt.Sprintf("kubernetes context %q was not found in %s", contextName, importResult.SourcePath),
		)
		return result
	}

	if contextName == "" {
		contextName = c.currentKubectlContext()
		if contextName == "" {
			contextName = currentImportedKubeContext(cfg.Profiles)
		}
		if contextName == "" {
			result.Warnings = append(result.Warnings, "could not determine the current kubectl context for deeper verification")
			return result
		}
	}

	namespace := strings.TrimSpace(profile.Kubernetes.Namespace)
	if namespace != "" {
		output, err := c.runKubectlLookup(contextName, "", "namespace", namespace)
		switch {
		case err != nil:
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf(
					"could not verify kubernetes namespace %q in context %q: %s",
					namespace,
					contextName,
					commandOutputSummary(output, err),
				),
			)
		case strings.TrimSpace(output) == "":
			result.Problems = append(
				result.Problems,
				fmt.Sprintf("kubernetes namespace %q was not found in context %q", namespace, contextName),
			)
		}
	}

	resourceType := strings.TrimSpace(profile.Kubernetes.ResourceType)
	if resourceType == "" || resourceName == "" {
		return result
	}

	output, err := c.runKubectlLookup(contextName, namespace, resourceType, resourceName)
	switch {
	case err != nil:
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"could not verify kubernetes %s %q%s: %s",
				resourceType,
				resourceName,
				kubeContextSuffix(contextName, namespace),
				commandOutputSummary(output, err),
			),
		)
	case strings.TrimSpace(output) == "":
		result.Problems = append(
			result.Problems,
			fmt.Sprintf(
				"kubernetes %s %q was not found%s",
				resourceType,
				resourceName,
				kubeContextSuffix(contextName, namespace),
			),
		)
	}

	return result
}

func (c *systemProfileProbeChecker) runKubectlLookup(contextName, namespace, resourceType, resourceName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"--request-timeout=5s"}
	if strings.TrimSpace(contextName) != "" {
		args = append(args, "--context", contextName)
	}
	if namespace != "" && resourceType != "namespace" {
		args = append(args, "--namespace", namespace)
	}
	args = append(args, "get", resourceType, resourceName, "-o", "name", "--ignore-not-found")

	output, err := c.runner.CombinedOutput(ctx, "kubectl", args)
	return string(output), err
}

func (c *systemProfileProbeChecker) currentKubectlContext() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := c.runner.CombinedOutput(ctx, "kubectl", []string{"config", "current-context"})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

type sshHostInspection struct {
	HostName string
}

func (c *systemProfileProbeChecker) inspectSSHHost(host string) (sshHostInspection, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := c.runner.CombinedOutput(ctx, "ssh", []string{"-G", host})
	return parseSSHHostInspection(string(output)), string(output), err
}

func parseSSHHostInspection(output string) sshHostInspection {
	inspection := sshHostInspection{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "hostname":
			if inspection.HostName == "" {
				inspection.HostName = strings.Join(fields[1:], " ")
			}
		}
	}
	return inspection
}

func checkSSHIdentityFiles(host string, identityFiles []string) []string {
	seen := make(map[string]struct{}, len(identityFiles))
	warnings := make([]string, 0, len(identityFiles))
	for _, path := range identityFiles {
		path = strings.TrimSpace(path)
		if path == "" || strings.EqualFold(path, "none") || strings.Contains(path, "%") {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}

		resolvedPath, ok := resolveSSHIdentityPath(path)
		if !ok {
			continue
		}
		if _, err := os.Stat(resolvedPath); err == nil {
			continue
		} else if os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("configured SSH identity file %q for alias %q was not found on disk", path, host))
		} else {
			warnings = append(warnings, fmt.Sprintf("could not inspect configured SSH identity file %q for alias %q: %v", path, host, err))
		}
	}
	return warnings
}

func resolveSSHIdentityPath(path string) (string, bool) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		if path == "~" {
			return homeDir, true
		}
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~/")), true
	}
	if filepath.IsAbs(path) {
		return path, true
	}
	return "", false
}

func profileProbeCacheKey(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		if profile.SSH == nil {
			return string(profile.Type) + ":" + profile.Name
		}
		return fmt.Sprintf("%s:%s:%s:%d", profile.Type, profile.SSH.Host, profile.SSH.RemoteHost, profile.SSH.RemotePort)
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote == nil {
			return string(profile.Type) + ":" + profile.Name
		}
		return fmt.Sprintf("%s:%s:%s:%d:%s:%d", profile.Type, profile.SSHRemote.Host, profile.SSHRemote.BindAddress, profile.SSHRemote.BindPort, profile.SSHRemote.TargetHost, profile.SSHRemote.TargetPort)
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic == nil {
			return string(profile.Type) + ":" + profile.Name
		}
		return fmt.Sprintf("%s:%s:%s:%d", profile.Type, profile.SSHDynamic.Host, profile.SSHDynamic.BindAddress, profile.LocalPort)
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes == nil {
			return string(profile.Type) + ":" + profile.Name
		}
		return fmt.Sprintf("%s:%s:%s:%s:%s:%d:%d", profile.Type, profile.Kubernetes.Context, profile.Kubernetes.Namespace, profile.Kubernetes.ResourceType, profile.Kubernetes.Resource, profile.Kubernetes.RemotePort, profile.LocalPort)
	default:
		return string(profile.Type) + ":" + profile.Name
	}
}

func sshHostForProfile(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		if profile.SSH != nil {
			return profile.SSH.Host
		}
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote != nil {
			return profile.SSHRemote.Host
		}
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic != nil {
			return profile.SSHDynamic.Host
		}
	}

	return ""
}

func looksLikeDirectSSHHost(host string) bool {
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return true
	}

	return strings.Contains(host, ".") || strings.Contains(host, ":") || strings.Contains(host, "@")
}

func hasNamedProfile(profiles []domain.Profile, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}

	return false
}

func currentImportedKubeContext(profiles []domain.Profile) string {
	for _, profile := range profiles {
		if hasLabel(profile.Labels, "current-context") {
			return profile.Name
		}
	}

	return ""
}

func commandOutputSummary(output string, err error) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return err.Error()
	}

	return output
}

func kubeContextSuffix(contextName, namespace string) string {
	if namespace != "" {
		return fmt.Sprintf(" in namespace %q for context %q", namespace, contextName)
	}
	return fmt.Sprintf(" for context %q", contextName)
}
