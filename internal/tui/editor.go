package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

var (
	editorMetaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("247"))
	editorActiveValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24")).
				Padding(0, 1)
	editorValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	editorLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("246")).
				Width(18)
	editorActiveLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("110")).
				Width(18)
	editorCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("220"))
)

type formEditorKind int

const (
	formEditorProfile formEditorKind = iota
	formEditorStack
)

type formFieldKind int

const (
	formFieldText formFieldKind = iota
	formFieldNumber
	formFieldList
	formFieldChoice
)

type formFieldOption struct {
	value string
	label string
}

type formField struct {
	key         string
	label       string
	help        string
	placeholder string
	required    bool
	kind        formFieldKind
	options     []formFieldOption
}

type formEditorState struct {
	kind       formEditorKind
	originName string
	focus      int
	values     map[string]string
	cursors    map[string]int
	fields     []formField
}

const (
	editorFieldName           = "name"
	editorFieldDescription    = "description"
	editorFieldLabels         = "labels"
	editorFieldType           = "type"
	editorFieldRestartEnabled = "restart_enabled"
	editorFieldMaxRetries     = "max_retries"
	editorFieldInitialBackoff = "initial_backoff"
	editorFieldMaxBackoff     = "max_backoff"
	editorFieldLocalPort      = "local_port"
	editorFieldHost           = "host"
	editorFieldRemoteHost     = "remote_host"
	editorFieldRemotePort     = "remote_port"
	editorFieldBindAddress    = "bind_address"
	editorFieldBindPort       = "bind_port"
	editorFieldTargetHost     = "target_host"
	editorFieldTargetPort     = "target_port"
	editorFieldContext        = "context"
	editorFieldNamespace      = "namespace"
	editorFieldResourceType   = "resource_type"
	editorFieldResource       = "resource"
	editorFieldProfiles       = "profiles"
	editorFieldStackMemberKey = "stack_member_"
)

func stackMemberFieldKey(index int) string {
	return fmt.Sprintf("%s%d", editorFieldStackMemberKey, index)
}

func isStackMemberFieldKey(key string) bool {
	return strings.HasPrefix(key, editorFieldStackMemberKey)
}

func stackMemberFieldIndex(key string) (int, bool) {
	if !isStackMemberFieldKey(key) {
		return 0, false
	}

	index, err := strconv.Atoi(strings.TrimPrefix(key, editorFieldStackMemberKey))
	if err != nil {
		return 0, false
	}
	return index, true
}

func newProfileEditorState(profile domain.Profile, originName string, language domain.Language) *formEditorState {
	profile = domain.PrepareProfileForType(profile, editableTunnelType(profile.Type))

	values := map[string]string{
		editorFieldName:           profile.Name,
		editorFieldDescription:    profile.Description,
		editorFieldLabels:         strings.Join(profile.Labels, ", "),
		editorFieldType:           string(profile.Type),
		editorFieldRestartEnabled: boolEditorValue(profile.Restart.Enabled),
		editorFieldMaxRetries:     strconv.Itoa(profile.Restart.MaxRetries),
		editorFieldInitialBackoff: profile.Restart.InitialBackoff,
		editorFieldMaxBackoff:     profile.Restart.MaxBackoff,
	}

	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		values[editorFieldHost] = profile.SSHRemote.Host
		values[editorFieldBindAddress] = profile.SSHRemote.BindAddress
		values[editorFieldBindPort] = strconv.Itoa(profile.SSHRemote.BindPort)
		values[editorFieldTargetHost] = profile.SSHRemote.TargetHost
		values[editorFieldTargetPort] = strconv.Itoa(profile.SSHRemote.TargetPort)
	case domain.TunnelTypeSSHDynamic:
		values[editorFieldHost] = profile.SSHDynamic.Host
		values[editorFieldBindAddress] = profile.SSHDynamic.BindAddress
		values[editorFieldLocalPort] = strconv.Itoa(profile.LocalPort)
	case domain.TunnelTypeKubernetesPortForward:
		values[editorFieldContext] = profile.Kubernetes.Context
		values[editorFieldNamespace] = profile.Kubernetes.Namespace
		values[editorFieldResourceType] = profile.Kubernetes.ResourceType
		values[editorFieldResource] = profile.Kubernetes.Resource
		values[editorFieldRemotePort] = strconv.Itoa(profile.Kubernetes.RemotePort)
		values[editorFieldLocalPort] = strconv.Itoa(profile.LocalPort)
	default:
		values[editorFieldHost] = profile.SSH.Host
		values[editorFieldRemoteHost] = profile.SSH.RemoteHost
		values[editorFieldRemotePort] = strconv.Itoa(profile.SSH.RemotePort)
		values[editorFieldLocalPort] = strconv.Itoa(profile.LocalPort)
	}

	editor := &formEditorState{
		kind:       formEditorProfile,
		originName: originName,
		values:     values,
		cursors:    make(map[string]int, len(values)),
	}
	for key, value := range values {
		editor.cursors[key] = runeLen(value)
	}
	editor.rebuild(language)
	editor.focusFieldByKey(recommendedProfileEditorField(profile))
	return editor
}

func newStackEditorState(stack domain.Stack, originName string, language domain.Language) *formEditorState {
	values := map[string]string{
		editorFieldName:        stack.Name,
		editorFieldDescription: stack.Description,
		editorFieldLabels:      strings.Join(stack.Labels, ", "),
	}

	editor := &formEditorState{
		kind:       formEditorStack,
		originName: originName,
		values:     values,
		cursors:    make(map[string]int, len(values)+len(stack.Profiles)),
	}
	for key, value := range values {
		editor.cursors[key] = runeLen(value)
	}
	editor.setStackMembersWithCursors(stack.Profiles, nil)
	editor.rebuild(language)
	editor.focusFieldByKey(recommendedStackEditorField(stack))
	return editor
}

func recommendedProfileEditorField(profile domain.Profile) string {
	if strings.TrimSpace(profile.Name) == "" {
		return editorFieldName
	}

	switch profile.Type {
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes == nil {
			return editorFieldContext
		}
		switch {
		case strings.TrimSpace(profile.Kubernetes.Context) == "":
			return editorFieldContext
		case strings.EqualFold(strings.TrimSpace(profile.Kubernetes.Resource), "change-me") || strings.TrimSpace(profile.Kubernetes.Resource) == "":
			return editorFieldResource
		case profile.Kubernetes.RemotePort == 0:
			return editorFieldRemotePort
		case profile.LocalPort == 0:
			return editorFieldLocalPort
		}
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote == nil {
			return editorFieldHost
		}
		switch {
		case strings.TrimSpace(profile.SSHRemote.Host) == "" || strings.TrimSpace(profile.SSHRemote.Host) == "example-bastion":
			return editorFieldHost
		case profile.SSHRemote.BindPort == 0:
			return editorFieldBindPort
		case strings.TrimSpace(profile.SSHRemote.TargetHost) == "":
			return editorFieldTargetHost
		case profile.SSHRemote.TargetPort == 0:
			return editorFieldTargetPort
		}
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic == nil {
			return editorFieldHost
		}
		switch {
		case strings.TrimSpace(profile.SSHDynamic.Host) == "" || strings.TrimSpace(profile.SSHDynamic.Host) == "example-bastion":
			return editorFieldHost
		case profile.LocalPort == 0:
			return editorFieldLocalPort
		}
	default:
		if profile.SSH == nil {
			return editorFieldHost
		}
		switch {
		case strings.TrimSpace(profile.SSH.Host) == "" || strings.TrimSpace(profile.SSH.Host) == "example-bastion":
			return editorFieldHost
		case strings.TrimSpace(profile.SSH.RemoteHost) == "" || strings.TrimSpace(profile.SSH.RemoteHost) == "127.0.0.1":
			return editorFieldRemoteHost
		case profile.SSH.RemotePort == 0 || (hasLabel(profile.Labels, "imported") && profile.SSH.RemotePort == 80):
			return editorFieldRemotePort
		case profile.LocalPort == 0:
			return editorFieldLocalPort
		}
	}

	return editorFieldName
}

func recommendedStackEditorField(stack domain.Stack) string {
	if len(stack.Profiles) == 0 {
		return stackMemberFieldKey(0)
	}
	if hasLabel(stack.Labels, "draft") {
		return stackMemberFieldKey(0)
	}
	return editorFieldName
}

func editableTunnelType(kind domain.TunnelType) domain.TunnelType {
	switch kind {
	case domain.TunnelTypeSSHRemote, domain.TunnelTypeSSHDynamic, domain.TunnelTypeKubernetesPortForward:
		return kind
	default:
		return domain.TunnelTypeSSHLocal
	}
}

func boolEditorValue(enabled bool) string {
	if enabled {
		return "true"
	}
	return "false"
}

func (e *formEditorState) rebuild(language domain.Language) {
	if e == nil {
		return
	}

	switch e.kind {
	case formEditorStack:
		e.fields = e.stackFields(language)
	default:
		profile := domain.PrepareProfileForType(e.profileDraft(), editableTunnelType(domain.TunnelType(strings.TrimSpace(e.values[editorFieldType]))))
		e.ensureProfileDefaults(profile)
		e.fields = e.profileFields(language, profile.Type)
	}

	if len(e.fields) == 0 {
		e.focus = 0
		return
	}
	e.focus = min(max(e.focus, 0), len(e.fields)-1)
	for _, field := range e.fields {
		e.cursors[field.key] = min(max(e.cursors[field.key], 0), runeLen(e.values[field.key]))
	}
}

func (e *formEditorState) stackFields(language domain.Language) []formField {
	members, cursors := e.stackMemberSnapshot()
	e.setStackMembersWithCursors(members, cursors)

	fields := []formField{
		{
			key:         editorFieldName,
			label:       translate(language, "Name", "名称"),
			help:        translate(language, "The stack name shown in the TUI and CLI.", "这个组合在 TUI 和 CLI 里的名字。"),
			placeholder: translate(language, "backend-dev", "backend-dev"),
			required:    true,
			kind:        formFieldText,
		},
		{
			key:         editorFieldDescription,
			label:       translate(language, "Description", "说明"),
			help:        translate(language, "Optional context so you remember what this stack is for.", "可选说明，帮助你记住这个组合是做什么的。"),
			placeholder: translate(language, "Daily backend stack", "日常后端组合"),
			kind:        formFieldText,
		},
		{
			key:         editorFieldLabels,
			label:       translate(language, "Labels", "标签"),
			help:        translate(language, "Comma-separated labels. Remove draft here when this stack is ready.", "逗号分隔的标签。组合准备好后，也可以在这里去掉 draft。"),
			placeholder: translate(language, "dev, api", "dev, api"),
			kind:        formFieldList,
		},
	}

	for idx := range members {
		fields = append(fields, formField{
			key:         stackMemberFieldKey(idx),
			label:       translatef(language, "Member %d", "成员 %d", idx+1),
			help:        translate(language, "Profile name in start order. + adds below, Ctrl+X removes, and [ or ] reorders.", "按启动顺序填写成员 profile 名。+ 在下方新增，Ctrl+X 删除，[ 或 ] 调整顺序。"),
			placeholder: translate(language, "prod-db", "prod-db"),
			required:    true,
			kind:        formFieldText,
		})
	}

	return fields
}

func (e *formEditorState) ensureProfileDefaults(profile domain.Profile) {
	e.ensureValue(editorFieldName, profile.Name)
	e.ensureValue(editorFieldDescription, profile.Description)
	e.ensureValue(editorFieldLabels, strings.Join(profile.Labels, ", "))
	e.ensureValue(editorFieldType, string(profile.Type))
	e.ensureValue(editorFieldRestartEnabled, boolEditorValue(profile.Restart.Enabled))
	e.ensureValue(editorFieldMaxRetries, strconv.Itoa(profile.Restart.MaxRetries))
	e.ensureValue(editorFieldInitialBackoff, profile.Restart.InitialBackoff)
	e.ensureValue(editorFieldMaxBackoff, profile.Restart.MaxBackoff)

	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		e.ensureValue(editorFieldHost, profile.SSHRemote.Host)
		e.ensureValue(editorFieldBindAddress, profile.SSHRemote.BindAddress)
		e.ensureValue(editorFieldBindPort, strconv.Itoa(profile.SSHRemote.BindPort))
		e.ensureValue(editorFieldTargetHost, profile.SSHRemote.TargetHost)
		e.ensureValue(editorFieldTargetPort, strconv.Itoa(profile.SSHRemote.TargetPort))
	case domain.TunnelTypeSSHDynamic:
		e.ensureValue(editorFieldHost, profile.SSHDynamic.Host)
		e.ensureValue(editorFieldBindAddress, profile.SSHDynamic.BindAddress)
		e.ensureValue(editorFieldLocalPort, strconv.Itoa(profile.LocalPort))
	case domain.TunnelTypeKubernetesPortForward:
		e.ensureValue(editorFieldContext, profile.Kubernetes.Context)
		e.ensureValue(editorFieldNamespace, profile.Kubernetes.Namespace)
		e.ensureValue(editorFieldResourceType, profile.Kubernetes.ResourceType)
		e.ensureValue(editorFieldResource, profile.Kubernetes.Resource)
		e.ensureValue(editorFieldRemotePort, strconv.Itoa(profile.Kubernetes.RemotePort))
		e.ensureValue(editorFieldLocalPort, strconv.Itoa(profile.LocalPort))
	default:
		e.ensureValue(editorFieldHost, profile.SSH.Host)
		e.ensureValue(editorFieldRemoteHost, profile.SSH.RemoteHost)
		e.ensureValue(editorFieldRemotePort, strconv.Itoa(profile.SSH.RemotePort))
		e.ensureValue(editorFieldLocalPort, strconv.Itoa(profile.LocalPort))
	}
}

func (e *formEditorState) profileDraft() domain.Profile {
	profile, err := e.profileFromValues(false)
	if err != nil {
		return domain.Profile{
			Type: editableTunnelType(domain.TunnelType(strings.TrimSpace(e.values[editorFieldType]))),
		}
	}
	return profile
}

func (e *formEditorState) ensureValue(key, value string) {
	if _, exists := e.values[key]; exists {
		return
	}
	e.values[key] = value
	e.cursors[key] = runeLen(value)
}

func (e *formEditorState) profileFields(language domain.Language, tunnelType domain.TunnelType) []formField {
	fields := []formField{
		{
			key:         editorFieldName,
			label:       translate(language, "Name", "名称"),
			help:        translate(language, "The profile name shown in the TUI and CLI.", "这个配置在 TUI 和 CLI 里的名字。"),
			placeholder: translate(language, "prod-db", "prod-db"),
			required:    true,
			kind:        formFieldText,
		},
		{
			key:         editorFieldDescription,
			label:       translate(language, "Description", "说明"),
			help:        translate(language, "Optional context so you remember what this tunnel is for.", "可选说明，帮助你记住这个隧道是做什么的。"),
			placeholder: translate(language, "Production database tunnel", "生产数据库隧道"),
			kind:        formFieldText,
		},
		{
			key:      editorFieldType,
			label:    translate(language, "Type", "类型"),
			help:     translate(language, "Use left/right or space to switch tunnel families.", "使用左右方向键或空格切换隧道类型。"),
			required: true,
			kind:     formFieldChoice,
			options: []formFieldOption{
				{value: string(domain.TunnelTypeSSHLocal), label: humanTunnelType(language, domain.TunnelTypeSSHLocal)},
				{value: string(domain.TunnelTypeSSHRemote), label: humanTunnelType(language, domain.TunnelTypeSSHRemote)},
				{value: string(domain.TunnelTypeSSHDynamic), label: humanTunnelType(language, domain.TunnelTypeSSHDynamic)},
				{value: string(domain.TunnelTypeKubernetesPortForward), label: humanTunnelType(language, domain.TunnelTypeKubernetesPortForward)},
			},
		},
		{
			key:         editorFieldLabels,
			label:       translate(language, "Labels", "标签"),
			help:        translate(language, "Comma-separated labels. Remove draft here when this profile is ready.", "逗号分隔的标签。配置准备好后，也可以在这里去掉 draft。"),
			placeholder: translate(language, "prod, db", "prod, db"),
			kind:        formFieldList,
		},
	}

	switch tunnelType {
	case domain.TunnelTypeSSHRemote:
		fields = append(fields,
			formField{
				key:         editorFieldHost,
				label:       translate(language, "SSH Host", "SSH 主机"),
				help:        translate(language, "SSH alias or hostname used for the remote forward.", "远程转发使用的 SSH alias 或主机名。"),
				placeholder: translate(language, "bastion-prod", "bastion-prod"),
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldBindAddress,
				label:       translate(language, "Bind Address", "监听地址"),
				help:        translate(language, "Optional remote bind address. Leave blank for the SSH server default.", "可选的远端监听地址。留空则使用 SSH 服务端默认值。"),
				placeholder: translate(language, "0.0.0.0", "0.0.0.0"),
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldBindPort,
				label:       translate(language, "Bind Port", "监听端口"),
				help:        translate(language, "Remote port exposed by the SSH server.", "SSH 服务端对外暴露的远端端口。"),
				placeholder: "9000",
				required:    true,
				kind:        formFieldNumber,
			},
			formField{
				key:         editorFieldTargetHost,
				label:       translate(language, "Target Host", "目标主机"),
				help:        translate(language, "Host reachable from the current machine when the remote forward connects back.", "远程转发回连时，当前机器上可被访问到的目标主机。"),
				placeholder: "127.0.0.1",
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldTargetPort,
				label:       translate(language, "Target Port", "目标端口"),
				help:        translate(language, "Port reachable from the current machine when the remote forward connects back.", "远程转发回连时，当前机器上可被访问到的目标端口。"),
				placeholder: "8080",
				required:    true,
				kind:        formFieldNumber,
			},
		)
	case domain.TunnelTypeSSHDynamic:
		fields = append(fields,
			formField{
				key:         editorFieldHost,
				label:       translate(language, "SSH Host", "SSH 主机"),
				help:        translate(language, "SSH alias or hostname used for the SOCKS tunnel.", "SOCKS 隧道使用的 SSH alias 或主机名。"),
				placeholder: translate(language, "bastion-dev", "bastion-dev"),
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldBindAddress,
				label:       translate(language, "Bind Address", "监听地址"),
				help:        translate(language, "Optional local bind address for the SOCKS listener.", "SOCKS 监听器的可选本地监听地址。"),
				placeholder: "127.0.0.1",
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldLocalPort,
				label:       translate(language, "Local Port", "本地端口"),
				help:        translate(language, "Local port opened for the SOCKS proxy.", "SOCKS 代理在本地打开的端口。"),
				placeholder: "1080",
				required:    true,
				kind:        formFieldNumber,
			},
		)
	case domain.TunnelTypeKubernetesPortForward:
		fields = append(fields,
			formField{
				key:         editorFieldContext,
				label:       translate(language, "Context", "Context"),
				help:        translate(language, "Optional kube context. Leave blank to use the current kubectl context.", "可选 kube context。留空则使用当前 kubectl context。"),
				placeholder: translate(language, "dev-cluster", "dev-cluster"),
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldNamespace,
				label:       translate(language, "Namespace", "命名空间"),
				help:        translate(language, "Optional namespace override. Leave blank for the context default.", "可选命名空间。留空则使用 context 默认值。"),
				placeholder: translate(language, "backend", "backend"),
				kind:        formFieldText,
			},
			formField{
				key:      editorFieldResourceType,
				label:    translate(language, "Resource Type", "资源类型"),
				help:     translate(language, "Use left/right or space to choose pod, service, or deployment.", "使用左右方向键或空格选择 pod、service 或 deployment。"),
				required: true,
				kind:     formFieldChoice,
				options: []formFieldOption{
					{value: "pod", label: "pod"},
					{value: "service", label: "service"},
					{value: "deployment", label: "deployment"},
				},
			},
			formField{
				key:         editorFieldResource,
				label:       translate(language, "Resource", "资源名称"),
				help:        translate(language, "Resource name to port-forward.", "要做端口转发的资源名称。"),
				placeholder: translate(language, "api", "api"),
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldRemotePort,
				label:       translate(language, "Remote Port", "远端端口"),
				help:        translate(language, "Port exposed by the Kubernetes resource.", "Kubernetes 资源对外暴露的端口。"),
				placeholder: "80",
				required:    true,
				kind:        formFieldNumber,
			},
			formField{
				key:         editorFieldLocalPort,
				label:       translate(language, "Local Port", "本地端口"),
				help:        translate(language, "Local port opened for kubectl port-forward.", "kubectl port-forward 在本地打开的端口。"),
				placeholder: "8080",
				required:    true,
				kind:        formFieldNumber,
			},
		)
	default:
		fields = append(fields,
			formField{
				key:         editorFieldHost,
				label:       translate(language, "SSH Host", "SSH 主机"),
				help:        translate(language, "SSH alias or hostname used for the tunnel.", "隧道使用的 SSH alias 或主机名。"),
				placeholder: translate(language, "bastion-prod", "bastion-prod"),
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldRemoteHost,
				label:       translate(language, "Remote Host", "远端主机"),
				help:        translate(language, "Target host reachable from the SSH server.", "SSH 服务端能够访问到的目标主机。"),
				placeholder: translate(language, "db.internal", "db.internal"),
				required:    true,
				kind:        formFieldText,
			},
			formField{
				key:         editorFieldRemotePort,
				label:       translate(language, "Remote Port", "远端端口"),
				help:        translate(language, "Target port reachable from the SSH server.", "SSH 服务端能够访问到的目标端口。"),
				placeholder: "5432",
				required:    true,
				kind:        formFieldNumber,
			},
			formField{
				key:         editorFieldLocalPort,
				label:       translate(language, "Local Port", "本地端口"),
				help:        translate(language, "Local port opened on this machine.", "在本机打开的本地端口。"),
				placeholder: "15432",
				required:    true,
				kind:        formFieldNumber,
			},
		)
	}

	fields = append(fields,
		formField{
			key:   editorFieldRestartEnabled,
			label: translate(language, "Restart", "自动重启"),
			help:  translate(language, "Use left/right or space to enable or disable automatic restarts.", "使用左右方向键或空格开启或关闭自动重启。"),
			kind:  formFieldChoice,
			options: []formFieldOption{
				{value: "true", label: translate(language, "Enabled", "开启")},
				{value: "false", label: translate(language, "Disabled", "关闭")},
			},
		},
		formField{
			key:         editorFieldMaxRetries,
			label:       translate(language, "Max Retries", "最大重试"),
			help:        translate(language, "0 means unlimited retries while restart is enabled.", "当自动重启开启时，0 表示不限次数。"),
			placeholder: "0",
			kind:        formFieldNumber,
		},
		formField{
			key:         editorFieldInitialBackoff,
			label:       translate(language, "Initial Backoff", "初始退避"),
			help:        translate(language, "Go duration like 2s or 500ms.", "使用 Go duration，例如 2s 或 500ms。"),
			placeholder: "2s",
			kind:        formFieldText,
		},
		formField{
			key:         editorFieldMaxBackoff,
			label:       translate(language, "Max Backoff", "最大退避"),
			help:        translate(language, "Go duration like 30s or 5m.", "使用 Go duration，例如 30s 或 5m。"),
			placeholder: "30s",
			kind:        formFieldText,
		},
	)

	return fields
}

func (e *formEditorState) currentField() (formField, bool) {
	if e == nil || len(e.fields) == 0 {
		return formField{}, false
	}
	return e.fields[min(max(e.focus, 0), len(e.fields)-1)], true
}

func (e *formEditorState) moveFocus(delta int) {
	if e == nil || len(e.fields) == 0 {
		return
	}
	e.focus = min(max(e.focus+delta, 0), len(e.fields)-1)
}

func (e *formEditorState) currentValue(field formField) string {
	if e == nil {
		return ""
	}
	return e.values[field.key]
}

func (e *formEditorState) setValue(key, value string) {
	e.values[key] = value
	e.cursors[key] = min(max(e.cursors[key], 0), runeLen(value))
}

func (e *formEditorState) cycleChoice(key string, delta int) {
	field, ok := e.fieldByKey(key)
	if !ok || len(field.options) == 0 {
		return
	}

	current := e.values[key]
	index := 0
	for idx, option := range field.options {
		if option.value == current {
			index = idx
			break
		}
	}

	index = (index + delta + len(field.options)) % len(field.options)
	e.values[key] = field.options[index].value
	e.cursors[key] = runeLen(e.values[key])
}

func (e *formEditorState) fieldByKey(key string) (formField, bool) {
	for _, field := range e.fields {
		if field.key == key {
			return field, true
		}
	}
	return formField{}, false
}

func (e *formEditorState) insertText(key, text string) {
	valueRunes := []rune(e.values[key])
	cursor := min(max(e.cursors[key], 0), len(valueRunes))
	insert := []rune(text)

	updated := make([]rune, 0, len(valueRunes)+len(insert))
	updated = append(updated, valueRunes[:cursor]...)
	updated = append(updated, insert...)
	updated = append(updated, valueRunes[cursor:]...)

	e.values[key] = string(updated)
	e.cursors[key] = cursor + len(insert)
}

func (e *formEditorState) backspace(key string) {
	valueRunes := []rune(e.values[key])
	cursor := min(max(e.cursors[key], 0), len(valueRunes))
	if cursor == 0 {
		return
	}

	e.values[key] = string(append(valueRunes[:cursor-1], valueRunes[cursor:]...))
	e.cursors[key] = cursor - 1
}

func (e *formEditorState) deleteForward(key string) {
	valueRunes := []rune(e.values[key])
	cursor := min(max(e.cursors[key], 0), len(valueRunes))
	if cursor >= len(valueRunes) {
		return
	}

	e.values[key] = string(append(valueRunes[:cursor], valueRunes[cursor+1:]...))
}

func (e *formEditorState) trimLastWord(key string) {
	valueRunes := []rune(e.values[key])
	cursor := min(max(e.cursors[key], 0), len(valueRunes))
	if cursor == 0 {
		return
	}

	left := string(valueRunes[:cursor])
	left = strings.TrimRight(left, " ")
	idx := strings.LastIndex(left, " ")
	if idx >= 0 {
		left = left[:idx+1]
	} else {
		left = ""
	}

	e.values[key] = left + string(valueRunes[cursor:])
	e.cursors[key] = runeLen(left)
}

func (e *formEditorState) clearValue(key string) {
	e.values[key] = ""
	e.cursors[key] = 0
}

func (e *formEditorState) moveCursor(key string, delta int) {
	e.cursors[key] = min(max(e.cursors[key]+delta, 0), runeLen(e.values[key]))
}

func (e *formEditorState) moveCursorToStart(key string) {
	e.cursors[key] = 0
}

func (e *formEditorState) moveCursorToEnd(key string) {
	e.cursors[key] = runeLen(e.values[key])
}

func (e *formEditorState) stackMemberFieldKeys() []string {
	if e == nil {
		return nil
	}

	keys := make([]string, 0)
	for idx := 0; ; idx++ {
		key := stackMemberFieldKey(idx)
		if _, exists := e.values[key]; !exists {
			break
		}
		keys = append(keys, key)
	}
	if len(keys) > 0 {
		return keys
	}

	for _, field := range e.fields {
		if isStackMemberFieldKey(field.key) {
			keys = append(keys, field.key)
		}
	}
	return keys
}

func (e *formEditorState) stackMemberSnapshot() ([]string, []int) {
	keys := e.stackMemberFieldKeys()
	if len(keys) == 0 {
		legacy := parseEditorList(e.values[editorFieldProfiles])
		if len(legacy) == 0 {
			legacy = []string{""}
		}
		cursors := make([]int, len(legacy))
		for idx, value := range legacy {
			cursors[idx] = runeLen(value)
		}
		return legacy, cursors
	}

	members := make([]string, 0, len(keys))
	cursors := make([]int, 0, len(keys))
	for _, key := range keys {
		value := e.values[key]
		members = append(members, value)
		cursors = append(cursors, min(max(e.cursors[key], 0), runeLen(value)))
	}
	if len(members) == 0 {
		return []string{""}, []int{0}
	}
	return members, cursors
}

func (e *formEditorState) setStackMembersWithCursors(members []string, cursors []int) {
	if e == nil {
		return
	}
	if len(members) == 0 {
		members = []string{""}
	}

	for key := range e.values {
		if key == editorFieldProfiles || isStackMemberFieldKey(key) {
			delete(e.values, key)
		}
	}
	for key := range e.cursors {
		if key == editorFieldProfiles || isStackMemberFieldKey(key) {
			delete(e.cursors, key)
		}
	}

	for idx, member := range members {
		key := stackMemberFieldKey(idx)
		e.values[key] = member
		cursor := runeLen(member)
		if idx < len(cursors) {
			cursor = min(max(cursors[idx], 0), runeLen(member))
		}
		e.cursors[key] = cursor
	}
}

func (e *formEditorState) focusFieldByKey(key string) {
	for idx, field := range e.fields {
		if field.key == key {
			e.focus = idx
			return
		}
	}
}

func (e *formEditorState) currentStackMemberIndex() (int, bool) {
	field, ok := e.currentField()
	if !ok {
		return 0, false
	}
	return stackMemberFieldIndex(field.key)
}

func (e *formEditorState) addStackMember(language domain.Language) {
	members, cursors := e.stackMemberSnapshot()
	insertAt := len(members)
	if index, ok := e.currentStackMemberIndex(); ok {
		insertAt = index + 1
	}

	members = append(members, "")
	cursors = append(cursors, 0)
	copy(members[insertAt+1:], members[insertAt:])
	copy(cursors[insertAt+1:], cursors[insertAt:])
	members[insertAt] = ""
	cursors[insertAt] = 0

	e.setStackMembersWithCursors(members, cursors)
	e.rebuild(language)
	e.focusFieldByKey(stackMemberFieldKey(insertAt))
}

func (e *formEditorState) removeCurrentStackMember(language domain.Language) {
	index, ok := e.currentStackMemberIndex()
	if !ok {
		return
	}

	members, cursors := e.stackMemberSnapshot()
	if len(members) <= 1 {
		e.setStackMembersWithCursors([]string{""}, []int{0})
		e.rebuild(language)
		e.focusFieldByKey(stackMemberFieldKey(0))
		return
	}

	members = append(members[:index], members[index+1:]...)
	cursors = append(cursors[:index], cursors[index+1:]...)
	e.setStackMembersWithCursors(members, cursors)
	e.rebuild(language)
	e.focusFieldByKey(stackMemberFieldKey(min(index, len(members)-1)))
}

func (e *formEditorState) moveCurrentStackMember(language domain.Language, delta int) {
	index, ok := e.currentStackMemberIndex()
	if !ok {
		return
	}

	members, cursors := e.stackMemberSnapshot()
	target := min(max(index+delta, 0), len(members)-1)
	if target == index {
		return
	}

	member := members[index]
	cursor := cursors[index]
	if target > index {
		copy(members[index:], members[index+1:target+1])
		copy(cursors[index:], cursors[index+1:target+1])
	} else {
		copy(members[target+1:], members[target:index])
		copy(cursors[target+1:], cursors[target:index])
	}
	members[target] = member
	cursors[target] = cursor

	e.setStackMembersWithCursors(members, cursors)
	e.rebuild(language)
	e.focusFieldByKey(stackMemberFieldKey(target))
}

func runeLen(value string) int {
	return len([]rune(value))
}

func (m Model) beginProfileEditor(profile domain.Profile, originName string) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.filterMode = false
	m.importMode = false
	m.pendingDelete = nil
	m.editor = newProfileEditorState(profile, originName, m.language())
	return m
}

func (m Model) beginStackEditor(stack domain.Stack, originName string) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.filterMode = false
	m.importMode = false
	m.pendingDelete = nil
	m.editor = newStackEditorState(stack, originName, m.language())
	return m
}

func (m Model) openSelectionEditor(profiles []app.ProfileView, stacks []app.StackView) Model {
	if m.focus == focusStacks && len(stacks) > 0 {
		selected := stacks[max(0, min(m.selectedStack, len(stacks)-1))]
		return m.beginStackEditor(cloneStackDefinition(selected.Stack), selected.Stack.Name)
	}

	if len(profiles) > 0 {
		selected := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
		return m.beginProfileEditor(cloneProfileDefinition(selected.Profile), selected.Profile.Name)
	}

	if m.workspaceIsEmpty() {
		return m.createStarterProfileDraft()
	}

	if m.filterQuery != "" {
		m.lastError = m.t("No visible item to edit. Clear the filter or select another item.", "当前没有可编辑的可见项。请清除筛选或换一个条目。")
		return m
	}

	m.lastError = m.t("Nothing is selected to edit yet.", "当前还没有选中任何可编辑的条目。")
	return m
}

func (m Model) renderEditorBody(width, height int) string {
	if m.editor == nil {
		return ""
	}

	innerWidth := panelInnerWidth(width)
	bodyHeight := panelBodyHeight(height)
	if bodyHeight <= 0 {
		return renderFixedPanel(m.editorTitle(), nil, width, height, true)
	}

	body := []string{
		editorMetaStyle.Render(truncateText(m.editorInstructionLine(), innerWidth)),
	}
	if guide := m.editorGuideLine(); guide != "" {
		body = append(body, editorMetaStyle.Render(truncateText(guide, innerWidth)))
	}
	body = append(body, editorMetaStyle.Render(truncateText(m.editorContextLine(), innerWidth)))

	fieldCount := len(m.editor.fields)
	visibleFields := max(1, bodyHeight-len(body)-1)
	start, end := windowAroundSelection(fieldCount, m.editor.focus, visibleFields)
	for idx := start; idx < end; idx++ {
		body = append(body, m.renderEditorField(m.editor.fields[idx], idx == m.editor.focus, innerWidth))
	}
	body = append(body, editorMetaStyle.Render(truncateText(m.editorHelpLine(), innerWidth)))

	return renderFixedPanel(m.editorTitle(), body, width, height, true)
}

func (m Model) editorTitle() string {
	if m.editor == nil {
		return m.t("Editor", "编辑器")
	}

	name := strings.TrimSpace(m.editor.values[editorFieldName])
	if name == "" {
		name = m.t("draft", "草稿")
	}

	switch m.editor.kind {
	case formEditorStack:
		return m.t("Stack Form ", "组合表单 ") + name
	default:
		return m.t("Profile Form ", "配置表单 ") + name
	}
}

func (m Model) editorInstructionLine() string {
	if m.editor != nil && m.editor.kind == formEditorStack {
		return m.t(
			"Up/Down or Tab move • type to edit • [ ] reorder member • + add member • Ctrl+X remove • Enter/Ctrl+S save • Esc cancel • E YAML",
			"上下或 Tab 移动 • 直接输入即可编辑 • [ ] 调整成员顺序 • + 新增成员 • Ctrl+X 删除 • Enter/Ctrl+S 保存 • Esc 取消 • E 打开 YAML",
		)
	}

	return m.t(
		"Up/Down or Tab move • type to edit • Left/Right moves or switches • Enter/Ctrl+S save • Esc cancel • E YAML",
		"上下或 Tab 移动 • 直接输入即可编辑 • 左右移动或切换 • Enter/Ctrl+S 保存 • Esc 取消 • E 打开 YAML",
	)
}

func (m Model) editorContextLine() string {
	if m.editor == nil {
		return ""
	}

	label := m.t("profile", "配置")
	if m.editor.kind == formEditorStack {
		label = m.t("stack", "组合")
	}

	return m.tf(
		"Editing %s %d/%d",
		"正在编辑%s %d/%d",
		label,
		m.editor.focus+1,
		max(1, len(m.editor.fields)),
	)
}

func (m Model) editorGuideLine() string {
	if m.editor == nil {
		return ""
	}

	switch m.editor.kind {
	case formEditorStack:
		labels := parseEditorList(m.editor.values[editorFieldLabels])
		if !hasLabel(labels, "draft") {
			return ""
		}
		fieldKey := recommendedStackEditorField(domain.Stack{Labels: labels, Profiles: stackEditorMembers(m.editor)})
		field, ok := m.editor.fieldByKey(fieldKey)
		if !ok {
			return m.t("Guide: finish the member list, then save and remove the draft label when ready.", "向导：先补完成员列表，准备好后保存并移除 draft 标签。")
		}
		return m.tf(
			"Guide: next fill %s, then save and remove the draft label when ready.",
			"向导：下一步先补 %s，准备好后保存并移除 draft 标签。",
			field.label,
		)
	default:
		profile := m.editor.profileDraft()
		if !hasLabel(profile.Labels, "draft") {
			return ""
		}
		field, ok := m.editor.fieldByKey(recommendedProfileEditorField(profile))
		if !ok {
			return m.t("Guide: finish the key target fields, then save and remove the draft label when ready.", "向导：先补完关键目标字段，准备好后保存并移除 draft 标签。")
		}
		return m.tf(
			"Guide: next fill %s, then save and remove the draft label when ready.",
			"向导：下一步先补 %s，准备好后保存并移除 draft 标签。",
			field.label,
		)
	}
}

func (m Model) editorHelpLine() string {
	if m.editor == nil {
		return ""
	}

	field, ok := m.editor.currentField()
	if !ok {
		return ""
	}
	help := field.help
	if m.editor.kind == formEditorStack && isStackMemberFieldKey(field.key) {
		if available := m.stackMemberSuggestionsLine(); available != "" {
			if help != "" {
				help += " "
			}
			help += available
		}
	}
	return help
}

func stackEditorMembers(editor *formEditorState) []string {
	if editor == nil {
		return nil
	}
	members, _ := editor.stackMemberSnapshot()
	return members
}

func (m Model) stackMemberSuggestionsLine() string {
	if m.service == nil {
		return ""
	}

	names := profileNames(m.service.Config())
	if len(names) == 0 {
		return ""
	}
	if len(names) > 6 {
		names = append(names[:6], "...")
	}

	return m.tf(
		"Available profiles: %s",
		"可用配置：%s",
		strings.Join(names, ", "),
	)
}

func (m Model) renderEditorField(field formField, active bool, width int) string {
	marker := "  "
	labelStyle := editorLabelStyle
	if active {
		marker = selectedMarkerStyle.Render("> ")
		labelStyle = editorActiveLabelStyle
	}

	label := field.label
	if field.required {
		label += " *"
	}

	prefix := marker + labelStyle.Render(truncateText(label, 18))
	valueWidth := max(1, width-lipgloss.Width(prefix)-1)
	return composeStyledLine(prefix+" ", m.renderEditorFieldValue(field, active, valueWidth), width)
}

func (m Model) renderEditorFieldValue(field formField, active bool, width int) string {
	value := m.editor.currentValue(field)

	switch field.kind {
	case formFieldChoice:
		label := value
		for _, option := range field.options {
			if option.value == value {
				label = option.label
				break
			}
		}
		if label == "" {
			label = field.placeholder
		}
		if active {
			return renderSizedBlock(editorActiveValueStyle, max(1, min(width, lipgloss.Width(label)+editorActiveValueStyle.GetHorizontalFrameSize())), truncateText(label, max(1, width-editorActiveValueStyle.GetHorizontalFrameSize())))
		}
		return editorValueStyle.Render(truncateText(label, width))
	default:
		if value == "" {
			return filterPlaceholderStyle.Render(truncateText(field.placeholder, width))
		}
		rendered := value
		if active {
			rendered = renderEditorCursor(value, m.editor.cursors[field.key])
			return editorActiveValueStyle.Render(truncateText(rendered, max(1, width-editorActiveValueStyle.GetHorizontalFrameSize())))
		}
		return editorValueStyle.Render(truncateText(rendered, width))
	}
}

func renderEditorCursor(value string, cursor int) string {
	runes := []rune(value)
	cursor = min(max(cursor, 0), len(runes))
	cursorToken := editorCursorStyle.Render(" ")
	if cursor < len(runes) {
		cursorToken = editorCursorStyle.Render(string(runes[cursor]))
		return string(runes[:cursor]) + cursorToken + string(runes[cursor+1:])
	}
	return string(runes) + cursorToken
}

func (m Model) handleEditorKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.editor == nil {
		return m, nil, false
	}

	field, ok := m.editor.currentField()
	if !ok {
		return m, nil, false
	}

	if m.editor.kind == formEditorStack {
		if _, isMemberField := m.editor.currentStackMemberIndex(); isMemberField {
			switch msg.String() {
			case "+":
				m.editor.addStackMember(m.language())
				return m, nil, true
			case "ctrl+x":
				m.editor.removeCurrentStackMember(m.language())
				return m, nil, true
			case "[":
				m.editor.moveCurrentStackMember(m.language(), -1)
				return m, nil, true
			case "]":
				m.editor.moveCurrentStackMember(m.language(), 1)
				return m, nil, true
			}
		}
	}

	switch msg.String() {
	case "esc":
		m.editor = nil
		m.lastNotice = m.t("Form edit cancelled.", "已取消表单编辑。")
		return m, nil, true
	case "enter", "ctrl+s":
		m = m.saveActiveEditor()
		return m, nil, true
	case "E":
		if err := m.ensureConfigFileExists(); err != nil {
			m.lastError = err.Error()
			return m, nil, true
		}
		m.editor = nil
		return m, openEditorCmd(m.configPath), true
	case "tab", "down":
		m.editor.moveFocus(1)
		return m, nil, true
	case "shift+tab", "up":
		m.editor.moveFocus(-1)
		return m, nil, true
	case "left":
		if field.kind == formFieldChoice {
			m.editor.cycleChoice(field.key, -1)
			if field.key == editorFieldType {
				m.editor.rebuild(m.language())
			}
		} else {
			m.editor.moveCursor(field.key, -1)
		}
		return m, nil, true
	case "right":
		if field.kind == formFieldChoice {
			m.editor.cycleChoice(field.key, 1)
			if field.key == editorFieldType {
				m.editor.rebuild(m.language())
			}
		} else {
			m.editor.moveCursor(field.key, 1)
		}
		return m, nil, true
	case "home":
		if field.kind != formFieldChoice {
			m.editor.moveCursorToStart(field.key)
		}
		return m, nil, true
	case "end":
		if field.kind != formFieldChoice {
			m.editor.moveCursorToEnd(field.key)
		}
		return m, nil, true
	case "backspace", "ctrl+h":
		if field.kind != formFieldChoice {
			m.editor.backspace(field.key)
		}
		return m, nil, true
	case "delete":
		if field.kind != formFieldChoice {
			m.editor.deleteForward(field.key)
		}
		return m, nil, true
	case "ctrl+w":
		if field.kind != formFieldChoice {
			m.editor.trimLastWord(field.key)
		}
		return m, nil, true
	case "ctrl+u":
		if field.kind != formFieldChoice {
			m.editor.clearValue(field.key)
		}
		return m, nil, true
	}

	switch msg.Type {
	case tea.KeySpace:
		if field.kind == formFieldChoice {
			m.editor.cycleChoice(field.key, 1)
			if field.key == editorFieldType {
				m.editor.rebuild(m.language())
			}
		} else {
			m.editor.insertText(field.key, " ")
		}
		return m, nil, true
	case tea.KeyRunes:
		if field.kind == formFieldChoice {
			needle := strings.ToLower(string(msg.Runes))
			for _, option := range field.options {
				if strings.HasPrefix(strings.ToLower(option.label), needle) || strings.HasPrefix(strings.ToLower(option.value), needle) {
					m.editor.setValue(field.key, option.value)
					if field.key == editorFieldType {
						m.editor.rebuild(m.language())
					}
					return m, nil, true
				}
			}
			return m, nil, true
		}

		m.editor.insertText(field.key, string(msg.Runes))
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m Model) saveActiveEditor() Model {
	if m.editor == nil {
		return m
	}

	m.lastError = ""
	m.lastNotice = ""

	switch m.editor.kind {
	case formEditorStack:
		return m.saveStackEditor()
	default:
		return m.saveProfileEditor()
	}
}

func (m Model) saveProfileEditor() Model {
	profile, err := m.editor.profileFromValues(true)
	if err != nil {
		m.lastError = err.Error()
		return m
	}

	cfg := m.service.Config()
	sourceName := m.editor.originName
	if sourceName != "" && sourceName != profile.Name {
		if configProfileExists(cfg, profile.Name) {
			m.lastError = m.t("Save profile: ", "保存配置失败: ") + fmt.Sprintf(m.t("profile %q already exists", "配置 %q 已存在"), profile.Name)
			return m
		}
		if !cfg.RemoveProfile(sourceName) {
			m.lastError = m.t("Save profile: ", "保存配置失败: ") + fmt.Sprintf(m.t("profile %q not found", "未找到配置 %q"), sourceName)
			return m
		}
	}

	cfg.SetProfile(profile)
	if sourceName != "" && sourceName != profile.Name {
		cfg.RenameProfileInStacks(sourceName, profile.Name)
	}

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Save profile: ", "保存配置失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.editor = nil
	m.filterMode = false
	m.filterQuery = ""
	m.focus = focusProfiles
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectProfileByName(profile.Name)
	m.lastNotice = m.tf("Saved profile %s.", "已保存配置 %s。", profile.Name)
	return m
}

func (m Model) saveStackEditor() Model {
	rawMembers, _ := m.editor.stackMemberSnapshot()
	nonEmptyMembers := 0
	for _, member := range rawMembers {
		if strings.TrimSpace(member) != "" {
			nonEmptyMembers++
		}
	}

	stack, err := m.editor.stackFromValues(true)
	if err != nil {
		m.lastError = err.Error()
		return m
	}

	cfg := m.service.Config()
	sourceName := m.editor.originName
	if sourceName != "" && sourceName != stack.Name {
		if configStackExists(cfg, stack.Name) {
			m.lastError = m.t("Save stack: ", "保存组合失败: ") + fmt.Sprintf(m.t("stack %q already exists", "组合 %q 已存在"), stack.Name)
			return m
		}
		if !cfg.RemoveStack(sourceName) {
			m.lastError = m.t("Save stack: ", "保存组合失败: ") + fmt.Sprintf(m.t("stack %q not found", "未找到组合 %q"), sourceName)
			return m
		}
	}

	if err := validateStackEditorMembers(cfg, stack); err != nil {
		m.lastError = m.t("Save stack: ", "保存组合失败: ") + err.Error()
		return m
	}

	cfg.SetStack(stack)
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Save stack: ", "保存组合失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.editor = nil
	m.filterMode = false
	m.filterQuery = ""
	m.focus = focusStacks
	m.selectedProfile = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectStackByName(stack.Name)
	if removed := nonEmptyMembers - len(stack.Profiles); removed > 0 {
		m.lastNotice = m.tf("Saved stack %s and removed %d duplicate member entries.", "已保存组合 %s，并去掉 %d 个重复成员。", stack.Name, removed)
		return m
	}
	m.lastNotice = m.tf("Saved stack %s.", "已保存组合 %s。", stack.Name)
	return m
}

func (e *formEditorState) profileFromValues(strict bool) (domain.Profile, error) {
	maxRetries := parseEditorIntLoose(e.values[editorFieldMaxRetries])
	if strict {
		var err error
		maxRetries, err = e.parseIntegerValue(editorFieldMaxRetries, true)
		if err != nil {
			return domain.Profile{}, err
		}
	}

	profileType := editableTunnelType(domain.TunnelType(strings.TrimSpace(e.values[editorFieldType])))
	profile := domain.Profile{
		Name:        strings.TrimSpace(e.values[editorFieldName]),
		Description: strings.TrimSpace(e.values[editorFieldDescription]),
		Type:        profileType,
		Labels:      parseEditorList(e.values[editorFieldLabels]),
		Restart: domain.RestartPolicy{
			Enabled:        strings.TrimSpace(e.values[editorFieldRestartEnabled]) != "false",
			MaxRetries:     maxRetries,
			InitialBackoff: strings.TrimSpace(e.values[editorFieldInitialBackoff]),
			MaxBackoff:     strings.TrimSpace(e.values[editorFieldMaxBackoff]),
		},
	}
	profile = domain.PrepareProfileForType(profile, profileType)

	switch profileType {
	case domain.TunnelTypeSSHRemote:
		bindPort, err := e.parsePortValue(editorFieldBindPort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		targetPort, err := e.parsePortValue(editorFieldTargetPort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		profile.LocalPort = bindPort
		profile.SSHRemote = &domain.SSHRemote{
			Host:        strings.TrimSpace(e.values[editorFieldHost]),
			BindAddress: strings.TrimSpace(e.values[editorFieldBindAddress]),
			BindPort:    bindPort,
			TargetHost:  strings.TrimSpace(e.values[editorFieldTargetHost]),
			TargetPort:  targetPort,
		}
	case domain.TunnelTypeSSHDynamic:
		localPort, err := e.parsePortValue(editorFieldLocalPort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		profile.LocalPort = localPort
		profile.SSHDynamic = &domain.SSHDynamic{
			Host:        strings.TrimSpace(e.values[editorFieldHost]),
			BindAddress: strings.TrimSpace(e.values[editorFieldBindAddress]),
		}
	case domain.TunnelTypeKubernetesPortForward:
		localPort, err := e.parsePortValue(editorFieldLocalPort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		remotePort, err := e.parsePortValue(editorFieldRemotePort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		profile.LocalPort = localPort
		profile.Kubernetes = &domain.Kubernetes{
			Context:      strings.TrimSpace(e.values[editorFieldContext]),
			Namespace:    strings.TrimSpace(e.values[editorFieldNamespace]),
			ResourceType: strings.TrimSpace(e.values[editorFieldResourceType]),
			Resource:     strings.TrimSpace(e.values[editorFieldResource]),
			RemotePort:   remotePort,
		}
	default:
		localPort, err := e.parsePortValue(editorFieldLocalPort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		remotePort, err := e.parsePortValue(editorFieldRemotePort, strict)
		if err != nil {
			return domain.Profile{}, err
		}
		profile.LocalPort = localPort
		profile.SSH = &domain.SSHLocal{
			Host:       strings.TrimSpace(e.values[editorFieldHost]),
			RemoteHost: strings.TrimSpace(e.values[editorFieldRemoteHost]),
			RemotePort: remotePort,
		}
	}

	if strict {
		if err := profile.Validate(); err != nil {
			return domain.Profile{}, err
		}
	}

	return profile, nil
}

func (e *formEditorState) stackFromValues(strict bool) (domain.Stack, error) {
	members, _ := e.stackMemberSnapshot()
	profiles := make([]string, 0, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		profiles = append(profiles, member)
	}
	profiles = dedupePreserveOrder(profiles)

	stack := domain.Stack{
		Name:        strings.TrimSpace(e.values[editorFieldName]),
		Description: strings.TrimSpace(e.values[editorFieldDescription]),
		Labels:      parseEditorList(e.values[editorFieldLabels]),
		Profiles:    profiles,
	}

	if strict {
		// The config save path validates against the actual profile list. Here we only
		// validate local shape so users get form feedback for empty values first.
		if strings.TrimSpace(stack.Name) == "" {
			return domain.Stack{}, fmt.Errorf("name is required")
		}
		if len(stack.Profiles) == 0 {
			return domain.Stack{}, fmt.Errorf("members must include at least one profile name")
		}
	}

	return stack, nil
}

func (e *formEditorState) parsePortValue(key string, strict bool) (int, error) {
	return e.parseIntegerValue(key, strict)
}

func (e *formEditorState) parseIntegerValue(key string, strict bool) (int, error) {
	value := strings.TrimSpace(e.values[key])
	if value == "" {
		if strict {
			field, _ := e.fieldByKey(key)
			return 0, fmt.Errorf("%s is required", field.label)
		}
		return 0, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		field, _ := e.fieldByKey(key)
		return 0, fmt.Errorf("%s must be a whole number", field.label)
	}
	return port, nil
}

func parseEditorIntLoose(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseEditorList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return cleaned
}

func dedupePreserveOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

func configProfileExists(cfg domain.Config, name string) bool {
	for _, profile := range cfg.Profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}

func configStackExists(cfg domain.Config, name string) bool {
	for _, stack := range cfg.Stacks {
		if stack.Name == name {
			return true
		}
	}
	return false
}

func validateStackEditorMembers(cfg domain.Config, stack domain.Stack) error {
	profileNames := make(map[string]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		profileNames[profile.Name] = struct{}{}
	}
	return stack.Validate(profileNames)
}
