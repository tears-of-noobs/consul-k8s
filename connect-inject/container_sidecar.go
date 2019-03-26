package connectinject

import (
	"bytes"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

type sidecarPreStopCommandData struct {
	ConsulAddr  string
}

func (h *Handler) containerSidecar(pod *corev1.Pod) corev1.Container {
	data := sidecarPreStopCommandData{
		ConsulAddr:  "${HOST_IP}:8500",
	}

	if h.UseTls {
		data.ConsulAddr = "https://${HOST_IP}:8501"
	}

	// Render the command
	var buf bytes.Buffer
	tpl := template.Must(template.New("root").Parse(strings.TrimSpace(
		sidecarPreStopCommand)))
	err := tpl.Execute(&buf, &data)
	if err != nil {
		return corev1.Container{}
	}
	return corev1.Container{
		Name:  "consul-connect-envoy-sidecar",
		Image: h.ImageEnvoy,
		Env: []corev1.EnvVar{
			{
				Name: "HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
				},
			},
		},
		VolumeMounts: h.volumeMounts(),
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-ec",
						strings.TrimSpace(sidecarPreStopCommand),
					},
				},
			},
		},
		Command: []string{
			"envoy",
			"--config-path", "/consul/connect-inject/envoy-bootstrap.yaml",
		},
	}
}

const sidecarPreStopCommand = `
export CONSUL_HTTP_ADDR="{{ .ConsulAddr }}"
export CONSUL_CACERT="/consul/tls/ca/tls.crt" 
export CONSUL_CLIENT_CERT="/consul/tls/client/tls.crt" 
export CONSUL_CLIENT_KEY="/consul/tls/client/tls.key"
export CONSUL_HTTP_SSL_VERIFY=false

/consul/connect-inject/consul services deregister \
  /consul/connect-inject/service.hcl
`
