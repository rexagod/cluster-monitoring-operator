// Copyright 2018 The Cluster Monitoring Operator Authors
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

package tasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/cluster-monitoring-operator/pkg/client"
	"github.com/openshift/cluster-monitoring-operator/pkg/manifests"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	annotationKSMCRSConfigEdit = "custom-resource-state/last-controller-config-edit"
	vpaGRString                = "verticalpodautoscalers.autoscaling.k8s.io"
)

type KubeStateMetricsTask struct {
	client  *client.Client
	factory *manifests.Factory
}

func NewKubeStateMetricsTask(client *client.Client, factory *manifests.Factory) *KubeStateMetricsTask {
	return &KubeStateMetricsTask{
		client:  client,
		factory: factory,
	}
}

func (t *KubeStateMetricsTask) Run(ctx context.Context) error {
	sa, err := t.factory.KubeStateMetricsServiceAccount()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics Service failed")
	}

	err = t.client.CreateOrUpdateServiceAccount(ctx, sa)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics ServiceAccount failed")
	}

	cr, err := t.factory.KubeStateMetricsClusterRole()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics ClusterRole failed")
	}

	err = t.client.CreateOrUpdateClusterRole(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics ClusterRole failed")
	}

	crb, err := t.factory.KubeStateMetricsClusterRoleBinding()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics ClusterRoleBinding failed")
	}

	err = t.client.CreateOrUpdateClusterRoleBinding(ctx, crb)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics ClusterRoleBinding failed")
	}

	rs, err := t.factory.KubeStateMetricsRBACProxySecret()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics RBAC proxy Secret failed")
	}

	err = t.client.CreateIfNotExistSecret(ctx, rs)
	if err != nil {
		return errors.Wrap(err, "creating kube-state-metrics RBAC proxy Secret failed")
	}

	svc, err := t.factory.KubeStateMetricsService()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics Service failed")
	}

	err = t.client.CreateOrUpdateService(ctx, svc)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics Service failed")
	}

	cm, err := t.factory.KubeStateMetricsCRSConfigMap()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics custom-resource-state ConfigMap failed")
	}

	// Check CRS prerequisites before enabling CRS metrics in CMO.
	// Currently, the prerequisites are:
	// * The presence of `VerticalPodAutoscaler` CRD in the cluster: `KubeStateMetricsListErrors` will be fired if absent.
	//   * If absent, modify the kube-state-metrics' custom-resource-state ConfigMap to disable CRS metrics.
	crsConfigPath := manifests.KubeStateMetricsCRSConfig
	_, err = t.client.ApiExtensionsInterface().ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), vpaGRString, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			i := strings.Index(crsConfigPath, "/")
			if i == -1 {
				return fmt.Errorf("unable to extract configuration filename from %s", crsConfigPath)
			}
			crsConfigFile := crsConfigPath[i+1:]
			// An empty custom-resource-state configuration file will crash KSM (for a non-empty `--custom-resource-state-config-file` flag value).
			cm.Data[crsConfigFile] = "kind: CustomResourceStateMetrics\n" // Empty kube-state-metrics' custom-resource-state configuration blob.
			return nil
		}
		return err
	}

	dep, err := t.factory.KubeStateMetricsDeployment()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics Deployment failed")
	}

	// If the kube-state-metrics' custom-resource-state ConfigMap has been modified, update the kube-state-metrics Deployment's annotations to trigger a rolling update.
	// TODO: This is a workaround to avoid a certain race condition upstream that crashes KSM whenever a configuration change is detected and causes the binary to reload.
	gotConfigMap, err := t.client.GetConfigmap(ctx, cm.Namespace, cm.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "getting %s/%s ConfigMap failed", cm.Namespace, cm.Name)
		}
	} else if gotConfigMap.Data[crsConfigPath] != cm.Data[crsConfigPath] {
		dep.Annotations[annotationKSMCRSConfigEdit] = time.Now().Format(time.RFC3339)
	}

	err = t.client.CreateOrUpdateConfigMap(ctx, cm)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics custom-resource-state ConfigMap failed")
	}

	err = t.client.CreateOrUpdateDeployment(ctx, dep)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics Deployment failed")
	}

	pr, err := t.factory.KubeStateMetricsPrometheusRule()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics rules PrometheusRule failed")
	}
	err = t.client.CreateOrUpdatePrometheusRule(ctx, pr)
	if err != nil {
		return errors.Wrap(err, "reconciling kube-state-metrics rules PrometheusRule failed")
	}

	sms, err := t.factory.KubeStateMetricsServiceMonitors()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics ServiceMonitors failed")
	}
	for _, sm := range sms {
		err = t.client.CreateOrUpdateServiceMonitor(ctx, sm)
		if err != nil {
			return errors.Wrapf(err, "reconciling %s/%s ServiceMonitor failed", sm.Namespace, sm.Name)
		}
	}

	return nil
}
