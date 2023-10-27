package manifests

import (
	_ "embed"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// //go:embed embedded/client.go
// var clientContents string

// //go:embed embedded/server.go
// var serverContents string

// type testingResources struct {
// 	Client  *appsv1.Deployment
// 	Server  *appsv1.Deployment
// 	Service *corev1.Service
// 	Ingress *netv1.Ingress
// }

// func (t testingResources) Objects() []client.Object {
// 	ret := []client.Object{
// 		t.Client,
// 		t.Server,
// 		t.Service,
// 		t.Ingress,
// 	}

// 	for _, obj := range ret {
// 		setGroupKindVersion(obj)
// 	}

// 	return ret
// }

func GetNginxServiceForTesting() *corev1.Service {
	//TODO: remove placholder annotation
	annotations := make(map[string]string)
	annotations["external-dns.alpha.kubernetes.io/hostname"] = "server.example.com"

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx-svc",
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Type:                  corev1.ServiceTypeLoadBalancer,
			Selector:              map[string]string{"app": "nginx"},
			Ports: []corev1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}

}
