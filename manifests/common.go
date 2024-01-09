package manifests

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ManagedByKey = "app.kubernetes.io/managed-by"
	// ManagedByVal is the value for the ManagedByKey label on all resources directly managed by our e2e tester
	ManagedByVal = "externalDNS-e2e"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	// add any types used in this package
	batchv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	metav1.AddMetaToScheme(scheme)
	appsv1.AddToScheme(scheme)
	policyv1.AddToScheme(scheme)
	rbacv1.AddToScheme(scheme)
}

// MarshalJson converts an object to json
func MarshalJson(obj client.Object) ([]byte, error) {
	codec := serializer.NewCodecFactory(scheme).LegacyCodec(scheme.PreferredVersionAllGroups()...)
	yml, err := runtime.Encode(codec, obj)
	if err != nil {
		return nil, fmt.Errorf("encoding object: %w", err)
	}

	return yml, nil
}
