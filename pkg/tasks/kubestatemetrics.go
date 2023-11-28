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
	"strings"

	"github.com/openshift/cluster-monitoring-operator/pkg/client"
	"github.com/openshift/cluster-monitoring-operator/pkg/manifests"
	"github.com/pkg/errors"
)

var (
	enableCRSMetricsArg = "custom-resource-state-config-file"
	crsConfigPath       = "/etc/kube-state-metrics/custom-resource-state-configmap.yaml"
)

type KubeStateMetricsTask struct {
	client           *client.Client
	factory          *manifests.Factory
	enableCRSMetrics bool
}

func NewKubeStateMetricsTask(client *client.Client, factory *manifests.Factory, enableCRSMetrics bool) *KubeStateMetricsTask {
	return &KubeStateMetricsTask{
		client:           client,
		factory:          factory,
		enableCRSMetrics: enableCRSMetrics,
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

	err = t.client.CreateOrUpdateConfigMap(ctx, cm)
	if err != nil {
		return errors.Wrapf(err, "reconciling %s/%s ConfigMap failed", cm.Namespace, cm.Name)
	}

	dep, err := t.factory.KubeStateMetricsDeployment()
	if err != nil {
		return errors.Wrap(err, "initializing kube-state-metrics Deployment failed")
	}

	// Check if custom-resource-state metrics have been enabled.
	// * If so, add the --custom-resource-state-config-file flag to the kube-state-metrics container, if it is absent.
	// * If not, remove the --custom-resource-state-config-file flag from the kube-state-metrics container, if it is present.
	// This will, in turn, also cause the kube-state-metrics container to be restarted with the new set of flags in effect.
	if t.enableCRSMetrics {
		for _, container := range dep.Spec.Template.Spec.Containers {
			if container.Name == "kube-state-metrics" {
				crsEnabled := false
				for _, arg := range container.Args {
					if strings.Contains(arg, enableCRSMetricsArg) {
						crsEnabled = true
						break
					}
				}
				if !crsEnabled {
					container.Args = append(container.Args, "--"+enableCRSMetricsArg+"="+crsConfigPath)
				}
				break
			}
		}
	} else {
		for _, container := range dep.Spec.Template.Spec.Containers {
			if container.Name == "kube-state-metrics" {
				for i, arg := range container.Args {
					if strings.Contains(arg, enableCRSMetricsArg) {
						container.Args = append(container.Args[:i], container.Args[i+1:]...)
						break
					}
				}
				break
			}
		}
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
