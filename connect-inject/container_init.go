package connectinject

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

type initContainerCommandData struct {
	ServiceName             string
	ServicePort             int32
	ServiceProtocol         string
	AuthMethod              string
	CentralConfig           bool
	EnvoyPrometheusBindAddr string
	Upstreams               []initContainerCommandUpstreamData
	Tags                    string
	ServiceTags             string
	Checks                  []initContainerServiceCheck
}

type initContainerCommandUpstreamData struct {
	Name        string
	LocalPort   int32
	Datacenter  string
	Query       string
	ServiceTags string
}

type initContainerServiceCheck struct {
	ID            string
	Name          string
	TCP           string
	HTTP          string
	TLSSkipVerify bool
	Method        string
	Interval      string
	Timeout       string
}

const fabioURLprefixTag = "urlprefix-"

// containerInit returns the init container spec for registering the Consul
// service, setting up the Envoy bootstrap, etc.
func (h *Handler) containerInit(pod *corev1.Pod) (corev1.Container, error) {
	data := initContainerCommandData{
		ServiceName:     pod.Annotations[annotationService],
		ServiceProtocol: pod.Annotations[annotationProtocol],
		AuthMethod:      h.AuthMethod,
		CentralConfig:   h.CentralConfig,
		Checks:          []initContainerServiceCheck{},
	}
	if data.ServiceName == "" {
		// Assertion, since we call defaultAnnotations above and do
		// not mutate pods without a service specified.
		panic("No service found. This should be impossible since we default it.")
	}

	// If a port is specified, then we determine the value of that port
	// and register that port for the host service.
	if raw, ok := pod.Annotations[annotationPort]; ok && raw != "" {
		if port, _ := portValue(pod, raw); port > 0 {
			data.ServicePort = port
		}
	}

	envoyPrometheusBindAddr, err := fetchEnvoyPrometheusBindAddr(pod)
	if err != nil {
		h.Log.Error(
			"can't fetch prometheus bind address for envoy proxy",
			"error message", err,
		)
		os.Exit(1)
	}

	data.EnvoyPrometheusBindAddr = envoyPrometheusBindAddr

	skipFabioTags := true

	raw, ok := pod.Annotations[annotationConnectSkipFabioTags]
	if ok {
		fabioTags, err := strconv.ParseBool(raw)
		if err != nil {
			h.Log.Error(
				"can't parse boolean value, set it to true forcely",
				"Value", raw,
			)
		} else {
			skipFabioTags = fabioTags
		}
	}

	// If tags are specified split the string into an array and create
	// the tags string
	if raw, ok := pod.Annotations[annotationTags]; ok && raw != "" {

		tags := strings.Split(raw, ",")

		var connectServiceTags, serviceTags []string
		for _, tag := range tags {
			if strings.HasPrefix(tag, fabioURLprefixTag) && skipFabioTags {
				serviceTags = append(serviceTags, tag)
				continue
			}
			connectServiceTags = append(connectServiceTags, tag)
			serviceTags = append(serviceTags, tag)
		}

		if len(connectServiceTags) != 0 {
			fmt.Println("kek")
			jsonTags, err := json.Marshal(connectServiceTags)
			if err != nil {
				h.Log.Error(
					"Error json marshaling connect service tags",
					"Error", err,
					"Tags", connectServiceTags,
				)
			}

			data.Tags = string(jsonTags)
		}

		if len(serviceTags) != 0 {
			jsonServiceTags, err := json.Marshal(serviceTags)
			if err != nil {
				h.Log.Error(
					"Error json marshaling service tags",
					"Error", err,
					"Tags", serviceTags,
				)
			}

			data.ServiceTags = string(jsonServiceTags)
		}

	}

	if raw, ok := pod.Annotations[annotationChecks]; ok && raw != "" {
		serviceChecks := []initContainerServiceCheck{}

		checks := strings.Split(raw, ",")

		for _, check := range checks {
			parts := strings.SplitN(check, ";", 8)

			serviceCheck := initContainerServiceCheck{
				Method:        http.MethodGet,
				TLSSkipVerify: false,
			}

			if len(parts) < 6 {
				panic(
					"incorrect check definition, it should be at least 6 fields",
				)
			}

			checkType := strings.TrimSpace(parts[0])

			switch checkType {
			case checkHTTP:
				serviceCheck.HTTP = strings.TrimSpace(parts[3])
			case checkTCP:
				serviceCheck.TCP = strings.TrimSpace(parts[3])
			default:
				panic(
					fmt.Sprintf(
						"unsupported check type %s",
						checkType,
					),
				)
			}

			serviceCheck.ID = fmt.Sprintf(
				"%s-%s",
				strings.ReplaceAll(data.ServiceName, " ", "-"),
				strings.TrimSpace(parts[1]),
			)

			serviceCheck.Name = strings.TrimSpace(parts[2])
			serviceCheck.Interval = strings.TrimSpace(parts[4])
			serviceCheck.Timeout = strings.TrimSpace(parts[5])

			if len(parts) == 8 {
				skipVerify, err := strconv.ParseBool(parts[7])
				if err != nil {
					h.Log.Error(
						"Error parse boolean value",
						"Error", err, "Value", parts[7],
					)
				}

				serviceCheck.TLSSkipVerify = skipVerify
				if strings.TrimSpace(parts[6]) != "" {
					serviceCheck.Method = strings.TrimSpace(parts[6])
				}
			}

			if len(parts) == 7 {
				serviceCheck.Method = strings.TrimSpace(parts[6])
			}

			serviceChecks = append(serviceChecks, serviceCheck)
		}

		data.Checks = serviceChecks
	}

	// If upstreams are specified, configure those
	if raw, ok := pod.Annotations[annotationUpstreams]; ok && raw != "" {
		for _, raw := range strings.Split(raw, ",") {
			parts := strings.SplitN(raw, ":", 4)

			var datacenter, service_name, prepared_query string
			var port int32
			var serviceTags []string
			if parts[0] == "prepared_query" {
				port, _ = portValue(pod, strings.TrimSpace(parts[2]))
				prepared_query = strings.TrimSpace(parts[1])
			} else {
				port, _ = portValue(pod, strings.TrimSpace(parts[1]))
				service_name = strings.TrimSpace(parts[0])

				// parse the optional datacenter
				if len(parts) > 2 {
					datacenter = strings.TrimSpace(parts[2])
				}

				// parse service tags
				if len(parts) > 3 {
					rawTags := strings.Split(parts[3], "#")
					for _, tag := range rawTags {
						serviceTags = append(
							serviceTags,
							strings.TrimSpace(tag),
						)
					}
				}
			}

			if port > 0 {
				jsonServiceTags, err := json.Marshal(serviceTags)
				if err != nil {
					h.Log.Error(
						"Error json marshaling service tags",
						"Error", err,
						"Tags", serviceTags,
					)
				}

				data.Upstreams = append(data.Upstreams, initContainerCommandUpstreamData{
					Name:        service_name,
					LocalPort:   port,
					Datacenter:  datacenter,
					Query:       prepared_query,
					ServiceTags: string(jsonServiceTags),
				})
			}
		}
	}

	// Create expected volume mounts
	volMounts := []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/consul/connect-inject",
		},
	}

	if h.AuthMethod != "" {
		// Extract the service account token's volume mount
		saTokenVolumeMount, err := findServiceAccountVolumeMount(pod)
		if err != nil {
			return corev1.Container{}, err
		}

		// Append to volume mounts
		volMounts = append(volMounts, saTokenVolumeMount)
	}

	// Render the command
	var buf bytes.Buffer
	tpl := template.Must(template.New("root").Parse(strings.TrimSpace(
		initContainerCommandTpl)))
	err = tpl.Execute(&buf, &data)
	if err != nil {
		return corev1.Container{}, err
	}

	return corev1.Container{
		Name:  "consul-connect-inject-init",
		Image: h.ImageConsul,
		Env: []corev1.EnvVar{
			{
				Name: "HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
				},
			},
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
				},
			},
		},
		VolumeMounts: volMounts,
		Command:      []string{"/bin/sh", "-ec", buf.String()},
	}, nil
}

// initContainerCommandTpl is the template for the command executed by
// the init container.
const initContainerCommandTpl = `
export CONSUL_HTTP_ADDR="${HOST_IP}:8500"
export CONSUL_GRPC_ADDR="${HOST_IP}:8502"

# Register the service. The HCL is stored in the volume so that
# the preStop hook can access it to deregister the service.
cat <<EOF >/consul/connect-inject/service.hcl
services {
  id   = "${POD_NAME}-{{ .ServiceName }}-sidecar-proxy"
  name = "{{ .ServiceName }}-sidecar-proxy"
  kind = "connect-proxy"
  address = "${POD_IP}"
  port = 20000
  {{- if .Tags}}
  tags = {{.Tags}}
  {{- end}}

  proxy {
    destination_service_name = "{{ .ServiceName }}"
    destination_service_id = "{{ .ServiceName}}"
    {{ if (gt .ServicePort 0) -}}
    local_service_address = "127.0.0.1"
    local_service_port = {{ .ServicePort }}
    {{ end -}}
	{{ if .EnvoyPrometheusBindAddr }}
	config {
      envoy_prometheus_bind_addr = "{{ .EnvoyPrometheusBindAddr }}"
    }
	{{ end }}


    {{ range .Upstreams -}}
    upstreams {
      {{- if .Name }}
      destination_type = "service" 
      destination_name = "{{ .Name }}"
      {{- if .ServiceTags }}
      destination_tags = {{ .ServiceTags }}
      {{- end }}
      {{- end }}
      {{- if .Query }}
      destination_type = "prepared_query" 
      destination_name = "{{ .Query}}"
      {{- end}}
      local_bind_port = {{ .LocalPort }}
      {{- if .Datacenter }}
      datacenter = "{{ .Datacenter }}"
      {{- end}}
    }
    {{ end }}
  }

  checks {
    name = "Proxy Public Listener"
    tcp = "${POD_IP}:20000"
    interval = "10s"
    deregister_critical_service_after = "10m"
  }

  checks {
    name = "Destination Alias"
    alias_service = "{{ .ServiceName }}"
  }
}

services {
  id   = "${POD_NAME}-{{ .ServiceName }}"
  name = "{{ .ServiceName }}"
  address = "${POD_IP}"
  port = {{ .ServicePort }}
  {{- if .ServiceTags }}
  tags = {{.ServiceTags}}
  {{- end}}

  {{ range $check := .Checks }}
  checks {
	id   = "${POD_NAME}-{{ $check.ID }}"
	name = "{{ $check.Name }}"
	{{- if $check.HTTP }}
	http = "{{ $check.HTTP }}"
	{{- end }}
	{{- if $check.TCP }}
	tcp  = "${POD_IP}:{{ $check.TCP }}"
	{{- end }}
	interval = "{{ $check.Interval }}"
	timeout = "{{ $check.Timeout }}"
	{{- if $check.HTTP }}
	{{- if $check.Method }}
	method = "{{ $check.Method }}"
	{{- end }}
	{{- if $check.TLSSkipVerify }}
	tls_skip_verify = {{ $check.TLSSkipVerify }}
	{{- end }}
	{{- end }}
  }
  {{ end }}
}
EOF

{{ if .CentralConfig -}}
# Create the central config's service registration
cat <<EOF >/consul/connect-inject/central-config.hcl
kind = "service-defaults"
name = "{{ .ServiceName }}"
protocol = "{{ .ServiceProtocol }}"
EOF
{{- end }}

{{ if .AuthMethod -}}
/bin/consul login -method="{{ .AuthMethod }}" \
  -bearer-token-file="/var/run/secrets/kubernetes.io/serviceaccount/token" \
  -token-sink-file="/consul/connect-inject/acl-token" \
  -meta="pod=${POD_NAMESPACE}/${POD_NAME}"
{{- end }}

{{ if .CentralConfig -}}
/bin/consul config write -cas -modify-index 0 \
  {{- if .AuthMethod }}
  -token-file="/consul/connect-inject/acl-token" \
  {{- end }}
  /consul/connect-inject/central-config.hcl || true
{{- end }}

/bin/consul services register \
  {{- if .AuthMethod }}
  -token-file="/consul/connect-inject/acl-token" \
  {{- end }}
  /consul/connect-inject/service.hcl

# Generate the envoy bootstrap code
/bin/consul connect envoy \
  -proxy-id="${POD_NAME}-{{ .ServiceName }}-sidecar-proxy" \
  {{- if .AuthMethod }}
  -token-file="/consul/connect-inject/acl-token" \
  {{- end }}
  -bootstrap > /consul/connect-inject/envoy-bootstrap.yaml

# Copy the Consul binary
cp /bin/consul /consul/connect-inject/consul
`
