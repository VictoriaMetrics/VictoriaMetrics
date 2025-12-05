local prometheusDatasource = {
  type: 'prometheus',
  uid: '${datasource}',
};

local alerts = std.parseJson(std.extVar('alerts'));
local renames = std.parseJson(std.extVar('renames'));

// Main alert queries - returns health percentage (0-100)
local targets = [
  {
    datasource: prometheusDatasource,
    editorMode: 'code',
    expr: a.expr,
    format: 'table',
    hide: false,
    instant: true,
    legendFormat: '{{svc_name}}',
    range: false,
    refId: a.refId,
  }
  for a in alerts
];

// Instance count query - shows number of instances per component (excludes logs/traces)
local instanceCountTarget = {
  datasource: prometheusDatasource,
  editorMode: 'code',
  expr: 'count by (svc_name) (label_replace(vm_app_version{job=~"$job", instance=~"$instance", version!~"(victoria-(logs|traces)|vl|vt).*"}, "svc_name", "$1", "version", "^(.+)-\\\\d{8}-.*"))',
  format: 'table',
  hide: false,
  instant: true,
  legendFormat: '{{svc_name}}',
  range: false,
  refId: 'InstanceCount',
};

local fieldConfig = {
  defaults: {
    color: { mode: 'thresholds' },
    custom: {
      align: 'center',
      cellOptions: { applyToRow: false, type: 'color-background' },
      filterable: true,
      inspect: false,
      minWidth: 80,
      wrapHeaderText: true,
      wrapText: false,
    },
    mappings: [
      {
        // 100% = all instances healthy
        options: {
          from: 100,
          result: { color: 'green', index: 0, text: '100%' },
          to: 100,
        },
        type: 'range',
      },
      {
        // 0-99% = some instances have issues (show actual %)
        options: {
          from: 0,
          result: { color: 'red', index: 1 },
          to: 99.99,
        },
        type: 'range',
      },
      {
        // Negative values = query error, show as 0%
        options: {
          from: -999999,
          result: { color: 'red', index: 2, text: 'ERR' },
          to: -0.01,
        },
        type: 'range',
      },
      {
        // N/A = alert not applicable to this component
        options: {
          match: 'null',
          result: { color: '#3D3D3D', index: 3, text: '-' },
        },
        type: 'special',
      },
    ],
    noValue: '-',
    thresholds: {
      mode: 'absolute',
      steps: [{ color: '#3D3D3D', value: null }, { color: 'red', value: 0 }, { color: 'green', value: 100 }],
    },
    unit: 'percent',
  },
  overrides: [
    // Alert column - don't color, just text
    {
      matcher: { id: 'byName', options: 'Alert' },
      properties: [
        { id: 'custom.cellOptions', value: { type: 'auto' } },
        { id: 'custom.width', value: 280 },
        { id: 'custom.filterable', value: true },
      ],
    },
  ],
};

// Instance count panel - table showing counts horizontally
local instanceCountFieldConfig = {
  defaults: {
    color: { mode: 'fixed', fixedColor: '#1F60C4' },
    custom: {
      align: 'center',
      cellOptions: { applyToRow: false, type: 'color-background' },
      filterable: false,
      inspect: false,
      minWidth: 80,
    },
    mappings: [],
    noValue: '-',
    thresholds: {
      mode: 'absolute',
      steps: [{ color: '#1F60C4', value: null }],
    },
    unit: 'none',
  },
  overrides: [
    // Hide the 'Metric' column header after transpose
    {
      matcher: { id: 'byName', options: 'Metric' },
      properties: [
        { id: 'custom.hidden', value: true },
      ],
    },
  ],
};

local transformations = [
  { id: 'merge', options: {} },
  {
    id: 'organize',
    options: {
      excludeByName: { Time: true },
      includeByName: {},
      indexByName: {},
      renameByName: renames + { svc_name: '' },
    },
  },
  { id: 'transpose', options: { firstFieldName: 'Alert', restFieldsName: '' } },
  { id: 'sortBy', options: { fields: {}, sort: [{ field: 'Alert' }] } },
];

// Transformations for instance count table
local instanceCountTransformations = [
  { id: 'merge', options: {} },
  {
    id: 'organize',
    options: {
      excludeByName: { Time: true },
      includeByName: {},
      indexByName: {},
    },
  },
  { id: 'transpose', options: { firstFieldName: 'Metric', restFieldsName: '' } },
  {
    id: 'organize',
    options: {
      excludeByName: { Metric: true },
      includeByName: {},
      indexByName: {},
    },
  },
];

local datasourceTemplate = {
  current: { text: 'default', value: 'default' },
  includeAll: false,
  label: 'Datasource',
  name: 'datasource',
  options: [],
  query: 'prometheus',
  refresh: 1,
  regex: '',
  type: 'datasource',
};

local jobTemplate = {
  allValue: '.*',
  current: { text: ['All'], value: ['$__all'] },
  datasource: prometheusDatasource,
  definition: 'label_values(vm_app_version, job)',
  includeAll: true,
  label: 'Job',
  multi: true,
  name: 'job',
  options: [],
  query: { query: 'label_values(vm_app_version, job)', refId: 'StandardVariableQuery' },
  refresh: 1,
  regex: '',
  sort: 1,
  type: 'query',
};

local instanceTemplate = {
  allValue: '.*',
  current: { text: ['All'], value: ['$__all'] },
  datasource: prometheusDatasource,
  definition: 'label_values(vm_app_version{job=~"$job"}, instance)',
  includeAll: true,
  label: 'Instance',
  multi: true,
  name: 'instance',
  options: [],
  query: { query: 'label_values(vm_app_version{job=~"$job"}, instance)', refId: 'StandardVariableQuery' },
  refresh: 1,
  regex: '',
  sort: 1,
  type: 'query',
};

local dashboardDescription = |||
  **VictoriaMetrics Status Page** - Health matrix for VictoriaMetrics components.

  **Reading the Table:**
  - **Instance Count** (Blue): Number of detected instances per component
  - **100%** (Green): All instances are healthy for this alert
  - **<100%** (Red): Some instances are experiencing issues (percentage shows healthy instances)
  - **-** (Gray): Alert not applicable to this component

  **Component Prefixes:**
  - **ALL:** Applies to all VictoriaMetrics components
  - **cluster:** Applies to vminsert, vmselect, vmstorage
  - **single:** Applies to victoria-metrics (single-node)
  - **vmagent/vmalert/vmauth/vmanomaly:** Component-specific alerts

  **Alert Rules Sources:**
  - [VictoriaMetrics Alerts Overview](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
  - [vmalert Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/vmalert.yml)
  - [vmagent Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/vmagent.yml)
  - [VM Cluster Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/cluster.yml)
  - [VM Single Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/single.yml)
  - [VM Operator Rules](https://github.com/VictoriaMetrics/operator/blob/master/config/alerting/vmoperator-rules.yaml)
  - [VMAnomaly Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/vmanomaly.yml)
|||;

{
  annotations: {
    list: [
      {
        builtIn: 1,
        datasource: { type: 'grafana', uid: '-- Grafana --' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotations & Alerts',
        type: 'dashboard',
      },
    ],
  },
  description: dashboardDescription,
  editable: true,
  fiscalYearStartMonth: 0,
  graphTooltip: 0,
  id: 0,
  links: [
    {
      asDropdown: false,
      icon: 'external link',
      includeVars: false,
      keepTime: false,
      tags: [],
      targetBlank: true,
      title: 'Alert Rules Source',
      tooltip: 'View official VictoriaMetrics alert rules on GitHub',
      type: 'link',
      url: 'https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/rules',
    },
  ],
  panels: [
    // Instance Count - Table panel (horizontal)
    {
      datasource: prometheusDatasource,
      description: 'Number of instances detected per component',
      fieldConfig: instanceCountFieldConfig,
      gridPos: { h: 4, w: 24, x: 0, y: 0 },
      id: 8000,
      options: {
        cellHeight: 'md',
        showHeader: true,
        footer: { show: false },
      },
      targets: [instanceCountTarget],
      title: 'Instance Count',
      transformations: instanceCountTransformations,
      type: 'table',
    },
    // Health Matrix
    {
      datasource: prometheusDatasource,
      description: |||
        Shows **worst health state** over the selected time range.
        
        **Values:** 100% = all healthy, <100% = issues detected, - = not applicable
        
        **Prefixes:** ALL = all components, cluster = vminsert/vmselect/vmstorage, single = victoria-metrics, or component-specific (vmagent, vmalert, vmauth, vmanomaly)
        
        **Sources:** [Alerts Overview](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts) | [cluster.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/cluster.yml) | [single.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/single.yml) | [vmagent.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/vmagent.yml) | [vmalert.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/rules/vmalert.yml)
      |||,
      fieldConfig: fieldConfig,
      gridPos: { h: 20, w: 24, x: 0, y: 4 },
      id: 9000,
      options: { 
        cellHeight: 'sm', 
        enablePagination: false, 
        showHeader: true,
        footer: {
          countRows: false,
          enablePagination: false,
          reducer: [],
          show: false,
        },
      },
      targets: targets,
      title: 'Service Health Matrix',
      transformations: transformations,
      type: 'table',
    },
  ],
  preload: false,
  refresh: '30s',
  schemaVersion: 42,
  tags: ['victoriametrics', 'status-page', 'alerts', 'health'],
  templating: { list: [datasourceTemplate, jobTemplate, instanceTemplate] },
  time: { from: 'now-5m', to: 'now' },
  timepicker: { refresh_intervals: ['10s', '30s', '1m', '5m'] },
  timezone: '',
  title: std.extVar('title'),
  uid: std.extVar('uid'),
  version: 1,
}
