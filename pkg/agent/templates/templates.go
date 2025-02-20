package templates

import (
	"bufio"
	"bytes"
	"io"
	"net/url"
	"strings"
	"text/template"

	"github.com/rancher/wharfie/pkg/registries"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
)

type ContainerdRuntimeConfig struct {
	RuntimeType string
	BinaryName  string
}

type ContainerdConfig struct {
	NodeConfig            *config.Node
	DisableCgroup         bool
	SystemdCgroup         bool
	IsRunningInUserNS     bool
	EnableUnprivileged    bool
	NoDefaultEndpoint     bool
	NonrootDevices        bool
	PrivateRegistryConfig *registries.Registry
	ExtraRuntimes         map[string]ContainerdRuntimeConfig
	Program               string
}

type RegistryEndpoint struct {
	OverridePath bool
	URL          *url.URL
	Rewrites     map[string]string
	Config       registries.RegistryConfig
}

type HostConfig struct {
	Default   *RegistryEndpoint
	Program   string
	Endpoints []RegistryEndpoint
}

// This version 2 config template is used by both Linux and Windows nodes
const ContainerdConfigTemplate = `
{{- /* */ -}}
# File generated by {{ .Program }}. DO NOT EDIT. Use config.toml.tmpl instead.
version = 2

[grpc]
  address = {{ deschemify .NodeConfig.Containerd.Address | printf "%q" }}

[plugins."io.containerd.internal.v1.opt"]
  path = "{{ .NodeConfig.Containerd.Opt }}"
[plugins."io.containerd.grpc.v1.cri"]
  stream_server_address = "127.0.0.1"
  stream_server_port = "10010"
  enable_selinux = {{ .NodeConfig.SELinux }}
  enable_unprivileged_ports = {{ .EnableUnprivileged }}
  enable_unprivileged_icmp = {{ .EnableUnprivileged }}
  device_ownership_from_security_context = {{ .NonrootDevices }}

{{- if .DisableCgroup}}
  disable_cgroup = true
{{end}}
{{- if .IsRunningInUserNS }}
  disable_apparmor = true
  restrict_oom_score_adj = true
{{end}}

{{- if .NodeConfig.AgentConfig.PauseImage }}
  sandbox_image = "{{ .NodeConfig.AgentConfig.PauseImage }}"
{{end}}

{{- if .NodeConfig.AgentConfig.Snapshotter }}
[plugins."io.containerd.grpc.v1.cri".containerd]
  snapshotter = "{{ .NodeConfig.AgentConfig.Snapshotter }}"
  disable_snapshot_annotations = {{ if or (eq .NodeConfig.AgentConfig.Snapshotter "stargz") (eq .NodeConfig.AgentConfig.Snapshotter "nix") }}false{{else}}true{{end}}
  {{ if .NodeConfig.DefaultRuntime }}default_runtime_name = "{{ .NodeConfig.DefaultRuntime }}"{{end}}
{{ if eq .NodeConfig.AgentConfig.Snapshotter "stargz" }}
{{ if .NodeConfig.AgentConfig.ImageServiceSocket }}
[plugins."io.containerd.snapshotter.v1.stargz"]
cri_keychain_image_service_path = "{{ .NodeConfig.AgentConfig.ImageServiceSocket }}"
[plugins."io.containerd.snapshotter.v1.stargz".cri_keychain]
enable_keychain = true
{{end}}

[plugins."io.containerd.snapshotter.v1.stargz".registry]
  config_path = "{{ .NodeConfig.Containerd.Registry }}"

{{ if .PrivateRegistryConfig }}
{{range $k, $v := .PrivateRegistryConfig.Configs }}
{{ if $v.Auth }}
[plugins."io.containerd.snapshotter.v1.stargz".registry.configs."{{$k}}".auth]
  {{ if $v.Auth.Username }}username = {{ printf "%q" $v.Auth.Username }}{{end}}
  {{ if $v.Auth.Password }}password = {{ printf "%q" $v.Auth.Password }}{{end}}
  {{ if $v.Auth.Auth }}auth = {{ printf "%q" $v.Auth.Auth }}{{end}}
  {{ if $v.Auth.IdentityToken }}identitytoken = {{ printf "%q" $v.Auth.IdentityToken }}{{end}}
{{end}}
{{end}}
{{end}}
{{end}}
{{ if eq .NodeConfig.AgentConfig.Snapshotter "nix" }}
[plugins."io.containerd.snapshotter.v1.nix"]
address = "{{ .NodeConfig.AgentConfig.ImageServiceSocket }}"
image_service.enable = true
[[plugins."io.containerd.transfer.v1.local".unpack_config]]
platform = "linux/amd64"
snapshotter = "nix"
{{end}}
{{end}}

{{- if or .NodeConfig.AgentConfig.CNIBinDir .NodeConfig.AgentConfig.CNIConfDir }}
[plugins."io.containerd.grpc.v1.cri".cni]
  {{ if .NodeConfig.AgentConfig.CNIBinDir }}bin_dir = {{ printf "%q" .NodeConfig.AgentConfig.CNIBinDir }}{{end}}
  {{ if .NodeConfig.AgentConfig.CNIConfDir }}conf_dir = {{ printf "%q" .NodeConfig.AgentConfig.CNIConfDir }}{{end}}
  bin_dir = "{{ .NodeConfig.AgentConfig.CNIBinDir }}"
  conf_dir = "{{ .NodeConfig.AgentConfig.CNIConfDir }}"
{{end}}

{{- if or .NodeConfig.Containerd.BlockIOConfig .NodeConfig.Containerd.RDTConfig }}
[plugins."io.containerd.service.v1.tasks-service"]
  {{ if .NodeConfig.Containerd.BlockIOConfig }}blockio_config_file = "{{ .NodeConfig.Containerd.BlockIOConfig }}"{{end}}
  {{ if .NodeConfig.Containerd.RDTConfig }}rdt_config_file = "{{ .NodeConfig.Containerd.RDTConfig }}"{{end}}
{{end}}

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
  SystemdCgroup = {{ .SystemdCgroup }}

[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "{{ .NodeConfig.Containerd.Registry }}"

{{ if .PrivateRegistryConfig }}
{{range $k, $v := .PrivateRegistryConfig.Configs }}
{{ if $v.Auth }}
[plugins."io.containerd.grpc.v1.cri".registry.configs."{{$k}}".auth]
  {{ if $v.Auth.Username }}username = {{ printf "%q" $v.Auth.Username }}{{end}}
  {{ if $v.Auth.Password }}password = {{ printf "%q" $v.Auth.Password }}{{end}}
  {{ if $v.Auth.Auth }}auth = {{ printf "%q" $v.Auth.Auth }}{{end}}
  {{ if $v.Auth.IdentityToken }}identitytoken = {{ printf "%q" $v.Auth.IdentityToken }}{{end}}
{{end}}
{{end}}
{{end}}

{{range $k, $v := .ExtraRuntimes}}
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."{{$k}}"]
  runtime_type = "{{$v.RuntimeType}}"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."{{$k}}".options]
  BinaryName = "{{$v.BinaryName}}"
  SystemdCgroup = {{ $.SystemdCgroup }}
{{end}}
`

// This version 3 config template is used by both Linux and Windows nodes
const ContainerdConfigTemplateV3 = `
{{- /* */ -}}
# File generated by {{ .Program }}. DO NOT EDIT. Use config.toml.tmpl instead.
version = 3
root = {{ printf "%q" .NodeConfig.Containerd.Root }}
state = {{ printf "%q" .NodeConfig.Containerd.State }}

[grpc]
  address = {{ deschemify .NodeConfig.Containerd.Address | printf "%q" }}

[plugins.'io.containerd.internal.v1.opt']
  path = {{ printf "%q" .NodeConfig.Containerd.Opt }}

[plugins.'io.containerd.grpc.v1.cri']
  stream_server_address = "127.0.0.1"
  stream_server_port = "10010"

[plugins.'io.containerd.cri.v1.runtime']
  enable_selinux = {{ .NodeConfig.SELinux }}
  enable_unprivileged_ports = {{ .EnableUnprivileged }}
  enable_unprivileged_icmp = {{ .EnableUnprivileged }}
  device_ownership_from_security_context = {{ .NonrootDevices }}

{{ if .DisableCgroup}}
  disable_cgroup = true
{{ end }}

{{ if .IsRunningInUserNS }}
  disable_apparmor = true
  restrict_oom_score_adj = true
{{ end }}

{{ with .NodeConfig.AgentConfig.Snapshotter }}
[plugins.'io.containerd.cri.v1.images']
  snapshotter = "{{ . }}"
  disable_snapshot_annotations = {{ if eq . "stargz" }}false{{else}}true{{end}}
{{ end }}

{{ with .NodeConfig.AgentConfig.PauseImage }}
[plugins.'io.containerd.cri.v1.images'.pinned_images]
  sandbox = "{{ . }}"
{{ end }}

[plugins.'io.containerd.cri.v1.images'.registry]
  config_path = {{ printf "%q" .NodeConfig.Containerd.Registry }}

{{- if or .NodeConfig.AgentConfig.CNIBinDir .NodeConfig.AgentConfig.CNIConfDir }}
[plugins.'io.containerd.cri.v1.runtime'.cni]
  {{ with .NodeConfig.AgentConfig.CNIConfDir }}conf_dir = {{ printf "%q" . }}{{ end }}
  {{ with .NodeConfig.AgentConfig.CNIConfDir }}conf_dir = {{ printf "%q" . }}{{ end }}
{{ end }}

{{ if or .NodeConfig.Containerd.BlockIOConfig .NodeConfig.Containerd.RDTConfig }}
[plugins.'io.containerd.service.v1.tasks-service']
  {{ with .NodeConfig.Containerd.BlockIOConfig }}blockio_config_file = {{ printf "%q" . }}{{ end }}
  {{ with .NodeConfig.Containerd.RDTConfig }}rdt_config_file = {{ printf "%q" . }}{{ end }}
{{ end }}

{{ with .NodeConfig.DefaultRuntime }}
[plugins.'io.containerd.cri.v1.runtime'.containerd]
  default_runtime_name = "{{ . }}"
{{ end }}

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc.options]
  SystemdCgroup = {{ .SystemdCgroup }}

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runhcs-wcow-process]
  runtime_type = "io.containerd.runhcs.v1"

{{ range $k, $v := .ExtraRuntimes }}
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.'{{ $k }}']
  runtime_type = "{{$v.RuntimeType}}"
{{ with $v.BinaryName}}
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.'{{ $k }}'.options]
  BinaryName = {{ printf "%q" . }}
  SystemdCgroup = {{ $.SystemdCgroup }}
{{ end }}
{{ end }}

[plugins.'io.containerd.cri.v1.images'.registry]
  config_path = {{ printf "%q" .NodeConfig.Containerd.Registry }}

{{ if .PrivateRegistryConfig }}
{{ range $k, $v := .PrivateRegistryConfig.Configs }}
{{ with $v.Auth }}
[plugins.'io.containerd.cri.v1.images'.registry.configs.'{{ $k }}'.auth]
  {{ with .Username }}username = {{ printf "%q" . }}{{ end }}
  {{ with .Password }}password = {{ printf "%q" . }}{{ end }}
  {{ with .Auth }}auth = {{ printf "%q" . }}{{ end }}
  {{ with .IdentityToken }}identitytoken = {{ printf "%q" . }}{{ end }}
{{ end }}
{{ end }}
{{ end }}

{{ if eq .NodeConfig.AgentConfig.Snapshotter "stargz" }}
{{ with .NodeConfig.AgentConfig.ImageServiceSocket }}
[plugins.'io.containerd.snapshotter.v1.stargz']
  cri_keychain_image_service_path = {{ printf "%q" . }}

[plugins.'io.containerd.snapshotter.v1.stargz'.cri_keychain]
  enable_keychain = true
{{ end }}

[plugins.'io.containerd.snapshotter.v1.stargz'.registry]
  config_path = {{ printf "%q" .NodeConfig.Containerd.Registry }}

{{ if .PrivateRegistryConfig }}
{{ range $k, $v := .PrivateRegistryConfig.Configs }}
{{ with $v.Auth }}
[plugins.'io.containerd.snapshotter.v1.stargz'.registry.configs.'{{ $k }}'.auth]
  {{ with .Username }}username = {{ printf "%q" . }}{{ end }}
  {{ with .Password }}password = {{ printf "%q" . }}{{ end }}
  {{ with .Auth }}auth = {{ printf "%q" . }}{{ end }}
  {{ with .IdentityToken }}identitytoken = {{ printf "%q" . }}{{ end }}
{{ end }}
{{ end }}
{{ end }}
{{ end }}
`

var HostsTomlHeader = "# File generated by " + version.Program + ". DO NOT EDIT.\n"

// This hosts.toml template is used by both Linux and Windows nodes
const HostsTomlTemplate = `
{{- /* */ -}}
# File generated by {{ .Program }}. DO NOT EDIT.
{{ with $e := .Default }}
{{- if $e.URL }}
server = "{{ $e.URL }}"
capabilities = ["pull", "resolve", "push"]
{{ end }}
{{- if $e.Config.TLS }}
{{- if $e.Config.TLS.CAFile }}
ca = [{{ printf "%q" $e.Config.TLS.CAFile }}]
{{- end }}
{{- if or $e.Config.TLS.CertFile $e.Config.TLS.KeyFile }}
client = [[{{ printf "%q" $e.Config.TLS.CertFile }}, {{ printf "%q" $e.Config.TLS.KeyFile }}]]
{{- end }}
{{- if $e.Config.TLS.InsecureSkipVerify }}
skip_verify = true
{{- end }}
{{ end }}
{{ end }}
[host]
{{ range $e := .Endpoints -}}
[host."{{ $e.URL }}"]
  capabilities = ["pull", "resolve"]
  {{- if $e.OverridePath }}
  override_path = true
  {{- end }}
{{- if $e.Config.TLS }}
  {{- if $e.Config.TLS.CAFile }}
  ca = [{{ printf "%q" $e.Config.TLS.CAFile }}]
  {{- end }}
  {{- if or $e.Config.TLS.CertFile $e.Config.TLS.KeyFile }}
  client = [[{{ printf "%q" $e.Config.TLS.CertFile }}, {{ printf "%q" $e.Config.TLS.KeyFile }}]]
  {{- end }}
  {{- if $e.Config.TLS.InsecureSkipVerify }}
  skip_verify = true
  {{- end }}
{{ end }}
{{- if $e.Rewrites }}
  [host."{{ $e.URL }}".rewrite]
  {{- range $pattern, $replace := $e.Rewrites }}
    "{{ $pattern }}" = "{{ $replace }}"
  {{- end }}
{{ end }}
{{ end -}}
`

func ParseTemplateFromConfig(userTemplate, baseTemplate string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(templateFuncs).Parse(userTemplate))
	template.Must(t.New("base").Parse(baseTemplate))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return trimEmpty(out)
}

func ParseHostsTemplateFromConfig(userTemplate string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(templateFuncs).Parse(userTemplate))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return trimEmpty(out)
}

// trimEmpty removes excess empty lines from the rendered template
func trimEmpty(r io.Reader) (string, error) {
	builder := strings.Builder{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			if strings.HasPrefix(line, "[") {
				builder.WriteString("\n")
			}
			builder.WriteString(line + "\n")
		}
	}
	return builder.String(), scanner.Err()
}
