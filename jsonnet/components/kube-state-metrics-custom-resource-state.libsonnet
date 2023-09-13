local vpaMetric(name, help, rawType, statesetScope='') = if name == '' || help == '' || rawType == '' then null else {
  local type = if rawType == 'StateSet' then 'stateSet' else std.asciiLower(rawType),
  local resourceType = if std.endsWith(name, 'cpu') then 'cpu' else if std.endsWith(name, 'memory') then 'memory' else '',
  local nameParts = std.split(name, '_'),
  local lastThree = [nameParts[i] for i in std.range(std.length(nameParts) - 3, std.length(nameParts) - 1)],
  local lastTwo = if lastThree[2] == type then [lastThree[0], lastThree[1]] else [lastThree[1], lastThree[2]],
  local defaultValueFrom = 1,
  local resolvedValueFrom = if resourceType == '' then null
  else {
    local noncamelcasedField = lastTwo[0],
    local camelcasedFields = {
      lowerbound: 'lowerBound',
      upperbound: 'upperBound',
      uncappedtarget: 'uncappedTarget',
      minallowed: 'minAllowed',
      maxallowed: 'maxAllowed',
    },
    local camelcasedField = if std.objectHas(camelcasedFields, noncamelcasedField) then camelcasedFields[noncamelcasedField] else noncamelcasedField,
    valueFrom: [camelcasedField, lastTwo[1]],
  },
  local includeAllFields = {
    annotations: ['metadata', 'annotations'],
    labels: ['metadata', 'labels'],
  },
  local commonLabelsFromPath = {
    namespace: ['metadata', 'namespace'],
    verticalpodautoscaler: ['metadata', 'name'],
    target_api_version: ['spec', 'targetRef', 'apiVersion'],
    target_kind: ['spec', 'targetRef', 'kind'],
    target_name: ['spec', 'targetRef', 'name'],
  },
  local shortPathMaps = {
    containerrecommendations: ['status', 'recommendation', 'containerRecommendations'],
    container_policies: ['spec', 'resourcePolicy', 'containerPolicies'],
    updatemode: ['spec', 'updatePolicy', 'updateMode'],
  },
  local shortPathMatches = [shortPathMaps[s] for s in std.objectFields(shortPathMaps) if std.length(std.findSubstr(s, name)) > 0],
  local label = if nameParts[std.length(nameParts) - 1] == type then nameParts[std.length(nameParts) - 2] else nameParts[std.length(nameParts) - 1],

  // spec.resources[*].groupVersionKind[*].metrics[*] (kube-state-metrics >=v2.5.0)
  name: name,
  help: help,
  commonLabels: if resourceType == '' then null else {
    resource: resourceType,
    unit: if resourceType == 'cpu' then 'cores' else if resourceType == 'memory' then 'bytes' else 'unknown',
  },
  each: {
    type: rawType,
    [type]: {
      [if std.length(shortPathMatches) > 1 then error 'expected 1 path match got ' + std.length(shortPathMatches) else if std.length(shortPathMatches) == 1 then 'path' else null]: shortPathMatches[0],
      // StateSets do not support internal labelsFromPath.
      [if type == 'stateSet' then null else 'labelsFromPath']: {
        container: ['containerName'],
        [if std.objectHas(includeAllFields, lastTwo[1]) then '*' else null]: includeAllFields[lastTwo[1]],
      },
      // labelName is only used by StateSets.
      [if type == 'stateSet' then 'labelName' else null]: label,
      // list is only used by StateSets.
      [if type == 'stateSet' && statesetScope != null then 'list' else null]: statesetScope,
      // valueFrom is only used by non-StateSets.
      [if type == 'stateSet' then null else 'valueFrom']: if resourceType == '' then defaultValueFrom else if resolvedValueFrom != null then resolvedValueFrom.valueFrom else null,
    },
  },
  labelsFromPath: commonLabelsFromPath,
};

local vpaMetrics = [
  vpaMetric('kube_verticalpodautoscaler_annotations_info', 'Kubernetes annotations converted to Prometheus labels.', 'Info'),
  vpaMetric('kube_verticalpodautoscaler_labels_info', 'Kubernetes labels converted to Prometheus labels.', 'Info'),
  vpaMetric('kube_verticalpodautoscaler_spec_updatepolicy_updatemode', 'Update mode of the VerticalPodAutoscaler.', 'StateSet', ['Off', 'Initial', 'Recreate', 'Auto']),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_cpu', 'Minimum cpu resources the container can use before the VerticalPodAutoscaler updater evicts it.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_memory', 'Minimum memory resources the container can use before the VerticalPodAutoscaler updater evicts it.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_cpu', 'Maximum cpu resources the container can use before the VerticalPodAutoscaler updater evicts it.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_memory', 'Maximum memory resources the container can use before the VerticalPodAutoscaler updater evicts it.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_target_cpu', 'Target cpu resources the VerticalPodAutoscaler recommends for the container.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_target_memory', 'Target memory resources the VerticalPodAutoscaler recommends for the container.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_cpu', 'Target cpu resources the VerticalPodAutoscaler recommends for the container ignoring bounds.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_memory', 'Target memory resources the VerticalPodAutoscaler recommends for the container ignoring bounds.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_minallowed_cpu', 'Minimum cpu resources the VerticalPodAutoscaler can set for containers matching the name.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_minallowed_memory', 'Minimum memory resources the VerticalPodAutoscaler can set for containers matching the name.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed_cpu', 'Minimum cpu resources the VerticalPodAutoscaler can set for containers matching the name.', 'Gauge'),
  vpaMetric('kube_verticalpodautoscaler_spec_resourcepolicy_container_policies_maxallowed_memory', 'Minimum memory resources the VerticalPodAutoscaler can set for containers matching the name.', 'Gauge'),
];

local crsConfig = {
  kind: 'CustomResourceStateMetrics',
  spec: {
    resources: [
      {
        groupVersionKind: {
          group: 'autoscaling.k8s.io',
          version: 'v1',
          kind: 'VerticalPodAutoscaler',
        },
        metrics: vpaMetrics,
      },
    ],
  },
};

{
  Config():: crsConfig,
}
