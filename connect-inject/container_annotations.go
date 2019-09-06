package connectinject

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/reconquest/karma-go"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultEnvoyPrometheusBindIP   = "0.0.0.0"
	DefaultEnvoyPrometheusBindPort = "9873"
)

func validateIPOrPort(
	ipAddress string,
	port string,
) error {
	destiny := karma.Describe(
		"method", "validateIPOrPort",
	)

	if ipAddress == "" && port == "" {
		return destiny.Describe(
			"ip address", ipAddress,
		).Describe(
			"port", port,
		).Reason(
			"ip address or port must not be empty",
		)
	}

	if ipAddress != "" {
		ipAddr := net.ParseIP(ipAddress)
		if ipAddr == nil {
			return destiny.Describe(
				"IP address definition", ipAddress,
			).Reason(
				"invalid IP address definition",
			)
		}
	}

	if port != "" {
		portValue, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			return destiny.Describe(
				"port definition", port,
			).Reason(
				"invalid port definition",
			)
		}
		portInValidRange := func(portValue int64) bool {
			if portValue >= 1024 && portValue <= 65535 {
				return true
			}

			return false
		}(portValue)

		if !portInValidRange {
			return destiny.Describe(
				"port definition", port,
			).Reason(
				"port value must be between 1024 and 65535",
			)
		}
	}

	return nil
}

func fetchEnvoyPrometheusBindAddr(
	pod *corev1.Pod,
) (string, error) {
	destiny := karma.Describe(
		"method", "fetchEnvoyPrometheusBindAddr",
	)
	defaultIP := DefaultEnvoyPrometheusBindIP
	defaultPort := DefaultEnvoyPrometheusBindPort

	err := validateIPOrPort(
		defaultIP,
		defaultPort,
	)
	if err != nil {
		return "", destiny.Describe(
			"default IP address", defaultIP,
		).Describe(
			"default port", defaultPort,
		).Describe(
			"error", err,
		).Reason(
			"can't validate IP or port",
		)
	}

	defaultAddress := fmt.Sprintf(
		"%s:%s",
		defaultIP,
		defaultPort,
	)

	rawValue, exists := pod.Annotations[annotationEnvoyPrometheusBindAddr]

	if !exists {
		return defaultAddress, nil
	}

	if rawValue == "" {
		return defaultAddress, nil
	}

	if strings.Contains(rawValue, ":") {
		parts := strings.SplitN(rawValue, ":", 2)

		if parts[0] != "" {
			err = validateIPOrPort(
				parts[0], "",
			)
			if err != nil {
				return "", destiny.Describe(
					"annotation IP definition", parts[0],
				).Describe(
					"error", err,
				).Reason(
					"can't validate annotation definition of IP address",
				)
			}

			defaultIP = parts[0]
		}

		if parts[1] != "" {
			err = validateIPOrPort(
				"", parts[1],
			)
			if err != nil {
				return "", destiny.Describe(
					"annotation port definition", parts[1],
				).Describe(
					"error", err,
				).Reason(
					"can't validate annotation definition of port",
				)
			}

			defaultPort = parts[1]
		}

		return fmt.Sprintf(
			"%s:%s",
			defaultIP,
			defaultPort,
		), nil

	}

	err = validateIPOrPort(
		"", rawValue,
	)
	if err != nil {
		return "", destiny.Describe(
			"annotation port definition", rawValue,
		).Describe(
			"error", err,
		).Reason(
			"can't validate annotation definition of port",
		)
	}

	return fmt.Sprintf(
		"%s:%s",
		defaultIP,
		rawValue,
	), nil
}
