// Copyright 2019 The Cluster Monitoring Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"fmt"
	v1 "k8s.io/api/autoscaling/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/openshift/cluster-monitoring-operator/test/e2e/framework"
	"github.com/pkg/errors"
)

func TestKSMMetricsSuppression(t *testing.T) {

	suppressedPattern, _ := regexp.Compile("kube_.*_annotations")

	err := framework.Poll(5*time.Second, time.Minute, func() error {

		client := f.PrometheusK8sClient

		b, err := client.PrometheusLabel("__name__")
		if err != nil {
			return err
		}

		response, err := gabs.ParseJSON(b)
		if err != nil {
			return err
		}

		status, ok := response.Path("status").Data().(string)
		if !ok {
			return errors.New("status not found")
		}

		if status != "success" {
			t.Errorf("Prometheus returned unexpected status: %s", status)
		}

		for _, name := range response.Search("data").Children() {
			metricName := name.Data().(string)
			if suppressedPattern.Match([]byte(metricName)) {
				t.Errorf("Metric should be suppressed: %s", metricName)
			}
		}

		return nil
	})
	if err != nil {
		t.Errorf("failed to query Prometheus: %v", err)
	}

}

func TestKSMCRSMetrics(t *testing.T) {
	// Install a VPAv1 CRD.
	manifest, err := f.ReadManifest("./assets/verticalpodautoscalers-v1-crd.yaml")
	if err != nil {
		t.Fatalf("failed to read VPA CRD manifest: %v", err)
	}
	vpaCRD, err := f.BuildCRD(manifest)
	if err != nil {
		t.Fatalf("failed to build VPA CRD: %v", err)
	}
	createVPACRD(t, vpaCRD)

	// Create a VPAv1 CR.
	vpaCR := &vpav1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vpa",
			Namespace: f.Ns,
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: vpav1.VerticalPodAutoscalerSpec{
			TargetRef: &v1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "cluster-monitoring-operator",
			},
		},
	}
	createVPACR(t, vpaCR)

	// Fetch KSM CRS metrics.
	err = framework.Poll(time.Second, time.Minute, func() error {
		response, err := f.PrometheusK8sClient.PrometheusQueryWithStatus("kube_customresource_kube_verticalpodautoscaler_labels_info", 200)
		if err != nil {
			return err
		}
		if !strings.Contains(string(response), "foo") {
			return errors.New(fmt.Sprintf("expected label 'foo' not found in response: %s", string(response)))
		}
		return nil
	})
	if err != nil {
		t.Errorf("failed to query Prometheus: %v", err)
	}

	// Cleanup.
	deleteVPACR(t, vpaCR)
	deleteVPACRD(t, vpaCRD)
}

func createVPACR(t *testing.T, vpaCR *vpav1.VerticalPodAutoscaler) {
	_, err := f.VPAClient.VerticalPodAutoscalers(f.Ns).Create(ctx, vpaCR, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create VPA CR: %v", err)
	}
	err = framework.Poll(time.Second, time.Minute, func() error {
		_, err = f.VPAClient.VerticalPodAutoscalers(f.Ns).Get(ctx, vpaCR.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create VPA CR: %v", err)
	}
}

func createVPACRD(t *testing.T, vpaCRD interface{}) {
	vpaCRD, err := f.APIExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, vpaCRD.(*apiextv1.CustomResourceDefinition), metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("failed to create VPA CRD: %v", err)
	}
	err = framework.Poll(time.Second, time.Minute, func() error {
		_, err = f.APIExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, vpaCRD.(*apiextv1.CustomResourceDefinition).Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to create VPA CRD: %v", err)
	}
}

func deleteVPACR(t *testing.T, vpaCR *vpav1.VerticalPodAutoscaler) {
	err := f.VPAClient.VerticalPodAutoscalers(f.Ns).Delete(ctx, vpaCR.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("failed to delete existing VPA CR: %v", err)
	}
	err = framework.Poll(time.Second, time.Minute, func() error {
		_, err := f.VPAClient.VerticalPodAutoscalers(f.Ns).Get(ctx, vpaCR.Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to delete existing VPA CR: %v", err)
	}
}

func deleteVPACRD(t *testing.T, vpaCRD interface{}) {
	err := f.APIExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, vpaCRD.(*apiextv1.CustomResourceDefinition).Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("failed to delete existing VPA CRD: %v", err)
	}
	err = framework.Poll(time.Second, time.Minute, func() error {
		_, err := f.APIExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, vpaCRD.(*apiextv1.CustomResourceDefinition).Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to delete existing VPA CRD: %v", err)
	}
}
