package clients

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// returns manifest for a basic nginx deployment
func NewNginxDeployment() *appsv1.Deployment {

	podLabels := make(map[string]string)
	podLabels["app"] = "nginx"

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: *WithPreferSystemNodes(&corev1.PodSpec{

					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				}),
			},
		},
	}
}

// returns nginx service
func NewNginxService() *corev1.Service {
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

func WithPreferSystemNodes(spec *corev1.PodSpec) *corev1.PodSpec {
	copy := spec.DeepCopy()
	copy.PriorityClassName = "system-node-critical"

	copy.Tolerations = append(copy.Tolerations, corev1.Toleration{
		Key:      "CriticalAddonsOnly",
		Operator: corev1.TolerationOpExists,
	})

	if copy.Affinity == nil {
		copy.Affinity = &corev1.Affinity{}
	}
	if copy.Affinity.NodeAffinity == nil {
		copy.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	copy.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(copy.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, corev1.PreferredSchedulingTerm{
		Weight: 100,
		Preference: corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key:      "kubernetes.azure.com/mode",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"system"},
			}},
		},
	})

	if copy.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		copy.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
	}
	copy.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(copy.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      "kubernetes.azure.com/cluster",
				Operator: corev1.NodeSelectorOpExists,
			},
			{
				Key:      "type",
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{"virtual-kubelet"},
			},
			{
				Key:      "kubernetes.io/os",
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			},
		},
	})

	return copy
}
