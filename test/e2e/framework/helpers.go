package framework

import (
	"io/ioutil"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	ClusterMonitorConfigMapName      = "cluster-monitoring-config"
	UserWorkloadMonitorConfigMapName = "user-workload-monitoring-config"

	E2eTestLabelName  = "app.kubernetes.io/created-by"
	E2eTestLabelValue = "cmo-e2e-test"
	E2eTestLabel      = E2eTestLabelName + ": " + E2eTestLabelValue
)

// BuildCMOConfigMap returns a ConfigMap holding the provided Cluster
// Monitoring Operator's config.
// If the config isn't valid YAML, the test will fail.
func (f *Framework) BuildCMOConfigMap(t *testing.T, config string) *v1.ConfigMap {
	t.Helper()

	cm, err := f.buildConfigMap(metav1.ObjectMeta{Name: ClusterMonitorConfigMapName, Namespace: f.Ns}, config)
	if err != nil {
		t.Fatal("invalid CMO config", err)
	}

	return cm
}

// BuildUserWorkloadConfigMap returns a ConfigMap holding the provided User
// Workload Monitoring's config.
// If the config isn't valid YAML, the test will fail.
func (f *Framework) BuildUserWorkloadConfigMap(t *testing.T, config string) *v1.ConfigMap {
	t.Helper()

	cm, err := f.buildConfigMap(metav1.ObjectMeta{Name: UserWorkloadMonitorConfigMapName, Namespace: f.UserWorkloadMonitoringNs}, config)
	if err != nil {
		t.Fatal("invalid user-workload config", err)
	}

	return cm
}

func (f *Framework) buildConfigMap(o metav1.ObjectMeta, config string) (*v1.ConfigMap, error) {
	s := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(config), &s)
	if err != nil {
		return nil, err
	}

	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: o.Namespace,
			Labels: map[string]string{
				E2eTestLabelName: E2eTestLabelValue,
			},
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}, nil
}

// ReadManifest reads a manifest from the provided path.
func (f *Framework) ReadManifest(manifestPath string) ([]byte, error) {
	manifestPath = filepath.Clean(manifestPath)
	return ioutil.ReadFile(manifestPath)
}

// BuildCRD builds a CRD from the provided manifest.
func (f *Framework) BuildCRD(manifest []byte) (runtime.Object, error) {
	crdStruct := &apiextv1.CustomResourceDefinition{}
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	got, _, err := decode(manifest, nil, crdStruct)
	if err != nil {
		return nil, err
	}
	return got, nil
}

// MustCreateOrUpdateConfigMap or fail the test
func (f *Framework) MustCreateOrUpdateConfigMap(t *testing.T, cm *v1.ConfigMap) {
	t.Helper()
	ensureCreatedByTestLabel(cm)
	err := Poll(time.Second, 10*time.Second, func() error {
		return f.OperatorClient.CreateOrUpdateConfigMap(ctx, cm)
	})
	if err != nil {
		t.Fatalf("failed to create/update configmap - %s", err.Error())
	}
}

// MustDeleteConfigMap or fail the test
func (f *Framework) MustDeleteConfigMap(t *testing.T, cm *v1.ConfigMap) {
	t.Helper()
	err := Poll(time.Second, 10*time.Second, func() error {
		return f.OperatorClient.DeleteConfigMap(ctx, cm)
	})
	if err != nil {
		t.Fatalf("failed to delete configmap - %s", err.Error())
	}
}

// MustGetConfigMap `name` from `namespace` within 5 minutes or fail
func (f *Framework) MustGetConfigMap(t *testing.T, name, namespace string) *v1.ConfigMap {
	t.Helper()
	var clusterCm *v1.ConfigMap
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		cm, err := f.KubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		clusterCm = cm
		return true, nil
	})
	if err != nil {
		t.Fatalf("failed to get configmap %s in namespace %s - %s", name, namespace, err.Error())
	}
	return clusterCm
}

// MustGetSecret `name` from `namespace` within 5 minutes or fail
func (f *Framework) MustGetSecret(t *testing.T, name, namespace string) *v1.Secret {
	t.Helper()
	var secret *v1.Secret
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		s, err := f.KubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		secret = s
		return true, nil
	})
	if err != nil {
		t.Fatalf("failed to get secret %s in namespace %s - %s", name, namespace, err.Error())
	}
	return secret
}

// MustGetStatefulSet `name` from `namespace` within 5 minutes or fail
func (f *Framework) MustGetStatefulSet(t *testing.T, name, namespace string) *appsv1.StatefulSet {
	t.Helper()
	var statefulSet *appsv1.StatefulSet
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		ss, err := f.KubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		statefulSet = ss
		return true, nil
	})
	if err != nil {
		t.Fatalf("failed to get statefulset %s in namespace %s - %s", name, namespace, err.Error())
	}
	return statefulSet
}

// MustGetPods return all pods from `namespace` within 5 minutes or fail
func (f *Framework) MustGetPods(t *testing.T, namespace string) *v1.PodList {
	t.Helper()
	var pods *v1.PodList
	err := wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
		pl, err := f.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}

		pods = pl
		return true, nil
	})
	if err != nil {
		t.Fatalf("failed to get pods in namespace %s - %s", namespace, err.Error())
	}
	return pods
}

func ensureCreatedByTestLabel(obj metav1.Object) {
	// only add the label if it doesn't exist yet, leave existing values
	// untouched
	labels := obj.GetLabels()
	if labels == nil {
		obj.SetLabels(map[string]string{
			E2eTestLabelName: E2eTestLabelValue,
		})
		return
	}

	if _, ok := labels[E2eTestLabelName]; !ok {
		labels[E2eTestLabelName] = E2eTestLabelValue
	}
}
