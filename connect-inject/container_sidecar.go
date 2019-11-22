package connectinject

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

func (h *Handler) containerSidecar(pod *corev1.Pod) (corev1.Container, error) {

	// Render the command
	var buf bytes.Buffer
	tpl := template.Must(template.New("root").Parse(strings.TrimSpace(
		sidecarPreStopCommandTpl)))
	err := tpl.Execute(&buf, h.AuthMethod)
	if err != nil {
		return corev1.Container{}, err
	}

	envoyPrometheusBindAddr, err := fetchEnvoyPrometheusBindAddr(pod)
	if err != nil {
		h.Log.Error(
			"can't fetch prometheus bind address for envoy proxy",
			"error message", err,
		)
		os.Exit(1)
	}

	// Validation already performed by fetchEnvoyPrometheusBindAddr
	// so on output we always have "host:port" value
	parts := strings.SplitN(envoyPrometheusBindAddr, ":", 2)
	port, _ := strconv.ParseInt(parts[1], 10, 64)

	envoyPrometheusPort := corev1.ContainerPort{
		Name:          "envoy-exporter",
		ContainerPort: int32(port),
	}

	return corev1.Container{
		Name:  "consul-connect-envoy-sidecar",
		Image: h.ImageEnvoy,
		Ports: []corev1.ContainerPort{envoyPrometheusPort},
		Env: []corev1.EnvVar{
			{
				Name: "HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				Name:      volumeName,
				MountPath: "/consul/connect-inject",
			},
		},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-ec",
						buf.String(),
					},
				},
			},
		},
		Command: []string{
			"envoy",
			"--config-path", "/consul/connect-inject/envoy-bootstrap.yaml",
		},
	}, nil
}

const sidecarPreStopCommandTpl = `
export CONSUL_HTTP_ADDR="${HOST_IP}:8500"
/consul/connect-inject/consul services deregister \
  {{- if . }}
  -token-file="/consul/connect-inject/acl-token" \
  {{- end }}
  /consul/connect-inject/service.hcl
{{- if . }}
&& /consul/connect-inject/consul logout \
  -token-file="/consul/connect-inject/acl-token"
{{- end}}
`
