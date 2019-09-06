package connectinject

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateIPOrPort(t *testing.T) {
	testCases := []struct {
		name     string
		testData struct {
			testIP   string
			testPort string
		}
		errorExpected bool
	}{
		{
			"check valid data",
			struct {
				testIP   string
				testPort string
			}{"127.0.0.1", "1024"},
			false,
		},
		{
			"check invalid IP",
			struct {
				testIP   string
				testPort string
			}{"127.0.0", "1024"},
			true,
		},
		{
			"check invalid Port",
			struct {
				testIP   string
				testPort string
			}{"127.0.0.1", "notvalidport"},
			true,
		},
		{
			"check empty values",
			struct {
				testIP   string
				testPort string
			}{"", ""},
			true,
		},
		{
			"check port in range",
			struct {
				testIP   string
				testPort string
			}{"", "30567"},
			false,
		},
		{
			"check port not in range",
			struct {
				testIP   string
				testPort string
			}{"", "999999"},
			true,
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				require := require.New(t)

				err := validateIPOrPort(
					testCase.testData.testIP,
					testCase.testData.testPort,
				)

				if testCase.errorExpected {
					require.Error(err)
					return
				}

				require.NoError(err)
			},
		)
	}
}

func TestFetchEnvoyPrometheusBindAddress(t *testing.T) {
	defaultAddress := fmt.Sprintf(
		"%s:%s",
		DefaultEnvoyPrometheusBindIP,
		DefaultEnvoyPrometheusBindPort,
	)

	testCases := []struct {
		name     string
		testData struct {
			testPod       *corev1.Pod
			expectedData  string
			expectedError bool
		}
	}{
		{
			"no annotation",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{},
				defaultAddress,
				false,
			},
		},
		{
			"empty annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "",
						},
					},
				},
				defaultAddress,
				false,
			},
		},
		{
			"valid IP:port annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "127.0.0.1:45678",
						},
					},
				},
				"127.0.0.1:45678",
				false,
			},
		},
		{
			"valid :port annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: ":45678",
						},
					},
				},
				fmt.Sprintf(
					"%s:45678",
					DefaultEnvoyPrometheusBindIP,
				),
				false,
			},
		},
		{
			"valid IP: annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "127.0.0.1:",
						},
					},
				},
				fmt.Sprintf(
					"127.0.0.1:%s",
					DefaultEnvoyPrometheusBindPort,
				),
				false,
			},
		},
		{
			"valid only port annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "45678",
						},
					},
				},
				fmt.Sprintf(
					"%s:45678",
					DefaultEnvoyPrometheusBindIP,
				),
				false,
			},
		},
		{
			"invalid IP:port annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "127.0.0:4567899",
						},
					},
				},
				"",
				true,
			},
		},
		{
			"invalid IP: annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "127.0.0:45678",
						},
					},
				},
				"",
				true,
			},
		},
		{
			"invalid :port annotation value",
			struct {
				testPod       *corev1.Pod
				expectedData  string
				expectedError bool
			}{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationEnvoyPrometheusBindAddr: "127.0.0.1:port",
						},
					},
				},
				"",
				true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				require := require.New(t)
				assert := assert.New(t)

				data, err := fetchEnvoyPrometheusBindAddr(
					testCase.testData.testPod,
				)

				assert.Equal(data, testCase.testData.expectedData)

				if testCase.testData.expectedError {
					require.Error(err)
					return
				}

				require.NoError(err)
			},
		)
	}

}
