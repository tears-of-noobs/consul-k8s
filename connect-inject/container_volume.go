package connectinject

import (
	corev1 "k8s.io/api/core/v1"
)

// volumeName is the name of the volume that is created to store the
// Consul Connect injection data.
const volumeName = "consul-connect-inject-data"

// containerVolume returns the volume data to add to the pod. This volume
// is used for shared data between containers.
func (h *Handler) containerVolume() corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (h *Handler) secretVolumes() []corev1.Volume {
	if !h.UseTls {
		return []corev1.Volume{}
	}
	return []corev1.Volume{
		corev1.Volume {
			Name: "tls-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: h.CaCertName,
				},
			},
		},
		corev1.Volume {
			Name: "tls-client-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: h.ClientCertName,
				},
			},
		},
	}
}

func (h *Handler) volumeMounts() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/consul/connect-inject",
		},
	}
	if h.UseTls {
		mounts = append(mounts,
			corev1.VolumeMount{
				Name:      "tls-ca-cert",
				MountPath: "/consul/tls/ca",
			},
			corev1.VolumeMount{
				Name: "tls-client-cert",
				MountPath: "/consul/tls/client",
			})
	}
	return mounts
}
