package generator

import "sort"

// AlertDefinition represents a normalized alert expression with refId.
type AlertDefinition struct {
	RefID string `json:"refId"`
	Expr  string `json:"expr"`
}

// RenderWithQuickTemplate builds the dashboard data and renders it via quicktemplate.
func RenderWithQuickTemplate(alerts []AlertDefinition, renames map[string]string, title, uid string) (string, error) {
	dashboard := BuildDashboard(alerts, renames, title, uid)
	return RenderDashboard(dashboard), nil
}

// BuildDashboard constructs the typed dashboard model that is rendered by quicktemplate.
func BuildDashboard(alerts []AlertDefinition, renames map[string]string, title, uid string) Dashboard {
	promDatasource := Datasource{Type: "prometheus", UID: "${datasource}"}

	targets := make([]Target, 0, len(alerts))
	for _, a := range alerts {
		targets = append(targets, Target{
			Datasource:   promDatasource,
			EditorMode:   "code",
			Expr:         a.Expr,
			Format:       "table",
			Hide:         false,
			Instant:      true,
			LegendFormat: "{{svc_name}}",
			Range:        false,
			RefID:        a.RefID,
		})
	}

	instanceCountTarget := Target{
		Datasource:   promDatasource,
		EditorMode:   "code",
		Expr:         `count by (svc_name) (label_replace(vm_app_version{job=~"$job", instance=~"$instance", version!~"(victoria-(logs|traces)|vl|vt).*"}, "svc_name", "$1", "version", "^(.+)-\\d{8}-.*"))`,
		Format:       "table",
		Hide:         false,
		Instant:      true,
		LegendFormat: "{{svc_name}}",
		Range:        false,
		RefID:        "InstanceCount",
	}

	// Typed field configuration for health matrix.
	fieldConfig := FieldConfig{
		Defaults: FieldDefaults{
			Color: Color{Mode: "thresholds"},
			Custom: CustomField{
				Align:          "center",
				CellOptions:    CellOptions{ApplyToRow: boolPtr(false), Type: "color-background"},
				Filterable:     true,
				Footer:         &Footer{Reducers: []string{"min"}},
				Inspect:        false,
				MinWidth:       80,
				WrapHeaderText: true,
				WrapText:       boolPtr(false),
			},
			Mappings: []Mapping{
				{
					Options: MappingOptions{
						From: floatPtr(100),
						To:   floatPtr(100),
						Result: MappingResult{
							Color: "green",
							Index: 0,
							Text:  strPtr("100%"),
						},
					},
					Type: "range",
				},
				{
					Options: MappingOptions{
						From: floatPtr(0),
						To:   floatPtr(99.99),
						Result: MappingResult{
							Color: "red",
							Index: 1,
						},
					},
					Type: "range",
				},
				{
					Options: MappingOptions{
						From: floatPtr(-999999),
						To:   floatPtr(-0.01),
						Result: MappingResult{
							Color: "red",
							Index: 2,
							Text:  strPtr("ERR"),
						},
					},
					Type: "range",
				},
				{
					Options: MappingOptions{
						Match: "null",
						Result: MappingResult{
							Color: "#3D3D3D",
							Index: 3,
							Text:  strPtr("-"),
						},
					},
					Type: "special",
				},
			},
			NoValue: "-",
			Thresholds: Thresholds{
				Mode: "absolute",
				Steps: []ThresholdStep{
					{Color: "#3D3D3D", Value: nil},
					{Color: "red", Value: floatPtr(0)},
					{Color: "green", Value: floatPtr(100)},
				},
			},
			Unit: "percent",
		},
		Overrides: []Override{
			{
				Matcher: Matcher{ID: "byName", Options: "Alert"},
				Properties: []Property{
					{ID: "custom.cellOptions", Value: CellOptions{Type: "auto"}},
					{ID: "custom.width", Value: 280},
					{ID: "custom.filterable", Value: true},
				},
			},
		},
	}

	instanceCountFieldConfig := FieldConfig{
		Defaults: FieldDefaults{
			Color: Color{Mode: "fixed", FixedColor: "#1F60C4"},
			Custom: CustomField{
				Align:       "center",
				CellOptions: CellOptions{ApplyToRow: boolPtr(false), Type: "color-background"},
				Filterable:  false,
				Inspect:     false,
				MinWidth:    80,
			},
			Mappings: []Mapping{},
			NoValue:  "-",
			Thresholds: Thresholds{
				Mode: "absolute",
				Steps: []ThresholdStep{
					{Color: "#1F60C4", Value: nil},
				},
			},
			Unit: "none",
		},
		Overrides: []Override{
			{
				Matcher: Matcher{ID: "byName", Options: "Metric"},
				Properties: []Property{
					{ID: "custom.hidden", Value: true},
				},
			},
		},
	}

	transformations := []Transformation{
		{ID: "merge", Options: MergeOptions{}},
		{
			ID: "organize",
			Options: OrganizeOptions{
				ExcludeByName: map[string]bool{"Time": true},
				IncludeByName: map[string]string{},
				IndexByName:   map[string]string{},
				RenameByName:  buildRenameByName(renames),
			},
		},
		{ID: "transpose", Options: TransposeOptions{FirstFieldName: "Alert", RestFieldsName: ""}},
		{ID: "sortBy", Options: SortByOptions{Fields: map[string]string{}, Sort: []SortField{{Field: "Alert"}}}},
	}

	instanceCountTransformations := []Transformation{
		{ID: "merge", Options: MergeOptions{}},
		{
			ID: "organize",
			Options: OrganizeOptions{
				ExcludeByName: map[string]bool{"Time": true},
				IncludeByName: map[string]string{},
				IndexByName:   map[string]string{},
			},
		},
		{ID: "transpose", Options: TransposeOptions{FirstFieldName: "Metric", RestFieldsName: ""}},
		{
			ID: "organize",
			Options: OrganizeOptions{
				ExcludeByName: map[string]bool{"Metric": true},
				IncludeByName: map[string]string{},
				IndexByName:   map[string]string{},
			},
		},
	}

	templates := []TemplateVar{
		{
			Current:    TemplateCurrent{Text: "default", Value: "default"},
			IncludeAll: false,
			Label:      "Datasource",
			Name:       "datasource",
			Options:    []string{},
			Query:      QueryString("prometheus"),
			Refresh:    1,
			Regex:      "",
			Type:       "datasource",
		},
		{
			AllValue:   ".*",
			Current:    TemplateCurrent{Text: []string{"All"}, Value: []string{"$__all"}},
			Datasource: &promDatasource,
			Definition: "label_values(vm_app_version, job)",
			IncludeAll: true,
			Label:      "Job",
			Multi:      true,
			Name:       "job",
			Options:    []string{},
			Query:      QueryTemplate(TemplateQuery{Query: "label_values(vm_app_version, job)", RefID: "StandardVariableQuery"}),
			Refresh:    1,
			Regex:      "",
			Sort:       1,
			Type:       "query",
		},
		{
			AllValue:   ".*",
			Current:    TemplateCurrent{Text: []string{"All"}, Value: []string{"$__all"}},
			Datasource: &promDatasource,
			Definition: `label_values(vm_app_version{job=~"$job"}, instance)`,
			IncludeAll: true,
			Label:      "Instance",
			Multi:      true,
			Name:       "instance",
			Options:    []string{},
			Query:      QueryTemplate(TemplateQuery{Query: `label_values(vm_app_version{job=~"$job"}, instance)`, RefID: "StandardVariableQuery"}),
			Refresh:    1,
			Regex:      "",
			Sort:       1,
			Type:       "query",
		},
	}

	desc := `**VictoriaMetrics Status Page** - Health matrix for VictoriaMetrics components.

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
`

	panels := []Panel{
		{
			Datasource:  promDatasource,
			Description: "Number of instances detected per component",
			FieldConfig: instanceCountFieldConfig,
			GridPos:     GridPos{H: 4, W: 24, X: 0, Y: 0},
			ID:          8000,
			Options: PanelOptions{
				CellHeight: "md",
				ShowHeader: true,
			},
			Targets:         []Target{instanceCountTarget},
			Title:           "Instance Count",
			Transformations: instanceCountTransformations,
			Type:            "table",
		},
		{
			Datasource:  promDatasource,
			Description: "Shows **worst health state** over the selected time range.\n\n**Values:** 100% = all healthy, <100% = issues detected, - = not applicable for this component\n\n**Prefixes:** ALL = all components, cluster = vminsert/vmselect/vmstorage, single = victoria-metrics, or component-specific (vmagent, vmalert, vmauth, vmanomaly)\n\n**Sources:** [Alerts Overview](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts) | [Alert Rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/rules)\n",
			FieldConfig: fieldConfig,
			GridPos:     GridPos{H: 20, W: 24, X: 0, Y: 4},
			ID:          9000,
			Options: PanelOptions{
				CellHeight:       "sm",
				EnablePagination: boolPtr(false),
				ShowHeader:       true,
			},
			Targets:         targets,
			Title:           "Service Health Matrix",
			Transformations: transformations,
			Type:            "table",
		},
	}

	return Dashboard{
		Annotations: Annotations{
			List: []AnnotationItem{
				{
					BuiltIn:    1,
					Datasource: Datasource{Type: "grafana", UID: "-- Grafana --"},
					Enable:     true,
					Hide:       true,
					IconColor:  "rgba(0, 211, 255, 1)",
					Name:       "Annotations & Alerts",
					Type:       "dashboard",
				},
			},
		},
		Description:          desc,
		Editable:             true,
		FiscalYearStartMonth: 0,
		GraphTooltip:         0,
		ID:                   0,
		Links: []Link{
			{
				AsDropdown:  false,
				Icon:        "external link",
				IncludeVars: false,
				KeepTime:    false,
				Tags:        []string{},
				TargetBlank: true,
				Title:       "Alert Rules Source",
				Tooltip:     "View official VictoriaMetrics alert rules on GitHub",
				Type:        "link",
				URL:         "https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/rules",
			},
		},
		Panels:        panels,
		Preload:       false,
		Refresh:       "30s",
		SchemaVersion: 42,
		Tags:          []string{"victoriametrics", "status-page", "alerts", "health"},
		Templating:    Templating{List: templates},
		Time:          TimeRange{From: "now-5m", To: "now"},
		Timepicker:    Timepicker{RefreshIntervals: []string{"10s", "30s", "1m", "5m"}},
		Timezone:      "",
		Title:         title,
		UID:           uid,
		Version:       1,
	}
}

func buildRenameByName(renames map[string]string) map[string]string {
	out := map[string]string{
		"svc_name": "",
	}

	keys := make([]string, 0, len(renames))
	for k := range renames {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = renames[k]
	}
	return out
}

func boolPtr(v bool) *bool {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func strPtr(v string) *string {
	return &v
}
