package generator

import "encoding/json"

// Top-level dashboard.
type Dashboard struct {
	Annotations          Annotations `json:"annotations"`
	Description          string      `json:"description"`
	Editable             bool        `json:"editable"`
	FiscalYearStartMonth int         `json:"fiscalYearStartMonth"`
	GraphTooltip         int         `json:"graphTooltip"`
	ID                   int         `json:"id"`
	Links                []Link      `json:"links"`
	Panels               []Panel     `json:"panels"`
	Preload              bool        `json:"preload"`
	Refresh              string      `json:"refresh"`
	SchemaVersion        int         `json:"schemaVersion"`
	Tags                 []string    `json:"tags"`
	Templating           Templating  `json:"templating"`
	Time                 TimeRange   `json:"time"`
	Timepicker           Timepicker  `json:"timepicker"`
	Timezone             string      `json:"timezone"`
	Title                string      `json:"title"`
	UID                  string      `json:"uid"`
	Version              int         `json:"version"`
}

type Annotations struct {
	List []AnnotationItem `json:"list"`
}

type AnnotationItem struct {
	BuiltIn    int        `json:"builtIn"`
	Datasource Datasource `json:"datasource"`
	Enable     bool       `json:"enable"`
	Hide       bool       `json:"hide"`
	IconColor  string     `json:"iconColor"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
}

type Link struct {
	AsDropdown  bool     `json:"asDropdown"`
	Icon        string   `json:"icon"`
	IncludeVars bool     `json:"includeVars"`
	KeepTime    bool     `json:"keepTime"`
	Tags        []string `json:"tags"`
	TargetBlank bool     `json:"targetBlank"`
	Title       string   `json:"title"`
	Tooltip     string   `json:"tooltip"`
	Type        string   `json:"type"`
	URL         string   `json:"url"`
}

type Panel struct {
	Datasource      Datasource       `json:"datasource"`
	Description     string           `json:"description"`
	FieldConfig     FieldConfig      `json:"fieldConfig"`
	GridPos         GridPos          `json:"gridPos"`
	ID              int              `json:"id"`
	Options         PanelOptions     `json:"options"`
	Targets         []Target         `json:"targets"`
	Title           string           `json:"title"`
	Transformations []Transformation `json:"transformations"`
	Type            string           `json:"type"`
}

type Datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type GridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

type PanelOptions struct {
	CellHeight       string  `json:"cellHeight"`
	EnablePagination *bool   `json:"enablePagination,omitempty"`
	ShowHeader       bool    `json:"showHeader"`
	Footer           *Footer `json:"footer,omitempty"`
}

type Footer struct {
	Show             *bool    `json:"show,omitempty"`
	CountRows        *bool    `json:"countRows,omitempty"`
	EnablePagination *bool    `json:"enablePagination,omitempty"`
	Reducers         []string `json:"reducers,omitempty"`
}

type FieldConfig struct {
	Defaults  FieldDefaults `json:"defaults"`
	Overrides []Override    `json:"overrides"`
}

type FieldDefaults struct {
	Color      Color       `json:"color"`
	Custom     CustomField `json:"custom"`
	Mappings   []Mapping   `json:"mappings"`
	NoValue    string      `json:"noValue"`
	Thresholds Thresholds  `json:"thresholds"`
	Unit       string      `json:"unit"`
}

type Color struct {
	Mode       string `json:"mode"`
	FixedColor string `json:"fixedColor,omitempty"`
}

type CustomField struct {
	Align          string      `json:"align"`
	CellOptions    CellOptions `json:"cellOptions"`
	Filterable     bool        `json:"filterable"`
	Footer         *Footer     `json:"footer,omitempty"`
	Inspect        bool        `json:"inspect"`
	MinWidth       int         `json:"minWidth"`
	WrapHeaderText bool        `json:"wrapHeaderText,omitempty"`
	WrapText       *bool       `json:"wrapText,omitempty"`
	Hidden         bool        `json:"hidden,omitempty"`
	Width          int         `json:"width,omitempty"`
}

type CellOptions struct {
	ApplyToRow *bool  `json:"applyToRow,omitempty"`
	Type       string `json:"type"`
}

type Mapping struct {
	Options MappingOptions `json:"options"`
	Type    string         `json:"type"`
}

type MappingOptions struct {
	From   *float64      `json:"from,omitempty"`
	To     *float64      `json:"to,omitempty"`
	Match  string        `json:"match,omitempty"`
	Result MappingResult `json:"result"`
}

type MappingResult struct {
	Color string  `json:"color"`
	Index int     `json:"index"`
	Text  *string `json:"text,omitempty"`
}

type Thresholds struct {
	Mode  string          `json:"mode"`
	Steps []ThresholdStep `json:"steps"`
}

type ThresholdStep struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

type Override struct {
	Matcher    Matcher    `json:"matcher"`
	Properties []Property `json:"properties"`
}

type Matcher struct {
	ID      string      `json:"id"`
	Options interface{} `json:"options"`
}

type Property struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
}

type Target struct {
	Datasource   Datasource `json:"datasource"`
	EditorMode   string     `json:"editorMode"`
	Expr         string     `json:"expr"`
	Format       string     `json:"format"`
	Hide         bool       `json:"hide"`
	Instant      bool       `json:"instant"`
	LegendFormat string     `json:"legendFormat"`
	Range        bool       `json:"range"`
	RefID        string     `json:"refId"`
}

type Transformation struct {
	ID      string      `json:"id"`
	Options interface{} `json:"options"`
}

type Templating struct {
	List []TemplateVar `json:"list"`
}

type TemplateVar struct {
	Current    TemplateCurrent    `json:"current"`
	IncludeAll bool               `json:"includeAll"`
	Label      string             `json:"label"`
	Name       string             `json:"name"`
	Options    []string           `json:"options,omitempty"`
	Query      TemplateQueryValue `json:"query"`
	Refresh    int                `json:"refresh"`
	Regex      string             `json:"regex"`
	Type       string             `json:"type"`
	AllValue   string             `json:"allValue,omitempty"`
	Datasource *Datasource        `json:"datasource,omitempty"`
	Definition string             `json:"definition,omitempty"`
	Multi      bool               `json:"multi,omitempty"`
	Sort       int                `json:"sort,omitempty"`
}

type TimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Timepicker struct {
	RefreshIntervals []string `json:"refresh_intervals"`
}

// Transformation option helpers.
type MergeOptions struct{}

type OrganizeOptions struct {
	ExcludeByName map[string]bool   `json:"excludeByName,omitempty"`
	IncludeByName map[string]string `json:"includeByName"`
	IndexByName   map[string]string `json:"indexByName"`
	RenameByName  map[string]string `json:"renameByName,omitempty"`
}

type TransposeOptions struct {
	FirstFieldName string `json:"firstFieldName"`
	RestFieldsName string `json:"restFieldsName"`
}

type SortByOptions struct {
	Fields map[string]string `json:"fields,omitempty"`
	Sort   []SortField       `json:"sort"`
}

type SortField struct {
	Field string `json:"field"`
}

// Template helpers.
type TemplateCurrent struct {
	Text  interface{} `json:"text"`
	Value interface{} `json:"value"`
}

type TemplateQuery struct {
	Query string `json:"query"`
	RefID string `json:"refId,omitempty"`
}

// TemplateQueryValue allows using either a raw string (datasource variable)
// or a structured query definition.
type TemplateQueryValue struct {
	String *string
	Query  *TemplateQuery
}

func QueryString(s string) TemplateQueryValue {
	return TemplateQueryValue{String: &s}
}

func QueryTemplate(q TemplateQuery) TemplateQueryValue {
	return TemplateQueryValue{Query: &q}
}

func (q TemplateQueryValue) MarshalJSON() ([]byte, error) {
	switch {
	case q.Query != nil:
		return json.Marshal(q.Query)
	case q.String != nil:
		return json.Marshal(*q.String)
	default:
		return json.Marshal(nil)
	}
}
