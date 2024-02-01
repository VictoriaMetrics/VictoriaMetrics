# vmui

Web UI for VictoriaMetrics

* [Docker image build](#docker-image-build)
* [Static build](#static-build)
* [Updating vmui embedded into VictoriaMetrics](#updating-vmui-embedded-into-victoriametrics)
* [Predefined dashboards](#predefined-dashboards)
* [App mode config options](#app-mode-config-options)
* [Timezone configuration](#timezone-configuration)

----

### Docker image build

Run the following command from the root of VictoriaMetrics repository in order to build `victoriametrics/vmui` Docker image:

```
make vmui-release
```

Then run the built image with:

```
docker run --rm --name vmui -p 8080:8080 victoriametrics/vmui
```

Then navigate to `http://localhost:8080` in order to see the web UI.


### Static build

Run the following command from the root of VictoriaMetrics repository for building `vmui` static contents:

```
make vmui-build
```

The built static contents is put into `app/vmui/packages/vmui/` directory.


### Updating vmui embedded into VictoriaMetrics

Run the following command from the root of VictoriaMetrics repository for updating `vmui` embedded into VictoriaMetrics:

```
make vmui-update
```

This command should update `vmui` static files at `app/vmselect/vmui` directory. Commit changes to these files if needed.

Then build VictoriaMetrics with the following command:

```
make victoria-metrics
```

Then run the built binary with the following command:

```
bin/victoria-metrics -selfScrapeInterval=5s
```

Then navigate to `http://localhost:8428/vmui/`. See [these docs](https://docs.victoriametrics.com/#vmui) for more details.

----


## Predefined dashboards

vmui allows you to pre-define dashboards. <br/>
Predefined dashboards will be displayed in the `"Dashboards"` tab.
If there are no dashboards or if they cannot be fetched, the tab will be hidden.

### Setup

You can setup pre-defined dashboards in two ways: [Setup by local files](#setup-by-local-files) or [Setup by flag](#setup-by-flag)

#### Setup by local files

By creating local files in the "folder" directory:

- In this case, to update the dashboards, you need to recompile the binary;
- These dashboards will be displayed if there are no dashboards located at the path set by the "--vmui.customDashboardsPath" flag.

1. Create `.json` config file in a folder `app/vmui/packages/vmui/public/dashboards/`
2. Add the name of the created JSON file to `app/vmui/packages/vmui/public/dashboards/index.js`

#### Setup by flag

It is possible to define path to the predefined dashboards by setting `--vmui.customDashboardsPath`.
- The panels are updated when the web page is loaded;
- Only those dashboards that are located at the path specified by the `--vmui.customDashboardsPath` flag are displayed;
- Local files from the previous step are ignored.

1. Single Version
   If you use single version of the VictoriaMetrics this flag should be provided for you execution file.
```
./victoria-metrics  --vmui.customDashboardsPath=/path/to/your/dashboards
```

2. Cluster Version
   If you use cluster version this flag should be defined for each `vmselect` component.
```
./vmselect -storageNode=:8418 --vmui.customDashboardsPath=/path/to/your/dashboards
```
At that moment all predefined dashboards files show be near each  `vmselect`. For example
if you have 3 `vmselect` instances you should create 3 copy of your predefined dashboards.

### Configuration options

DashboardSettings:

| Name      |       Type       |                Description |
|:----------|:----------------:|---------------------------:|
| rows*     | `DashboardRow[]` | Sections containing panels |
| title     |     `string`     |            Dashboard title |

<br/>
DashboardRow:

| Name       |       Type        |                Description |
|:-----------|:-----------------:|---------------------------:|
| panels*    | `PanelSettings[]` |    List of panels (charts) |
| title      |     `string`      |                  Row title |

<br/>
PanelSettings:

| Name        |    Type    |                                                                           Description |
|:------------|:----------:|--------------------------------------------------------------------------------------:|
| expr*       | `string[]` |                                                                   Data source queries |
| alias       | `string[]` |                                           Expression alias. Matched by index in array |
| title       |  `string`  |                                                                           Panel title |
| description |  `string`  |                                                Additional information about the panel |
| unit        |  `string`  |                                                                           Y-axis unit |
| showLegend  | `boolean`  |                                   If `false`, the legend hide. Default value - `true` |
| width       |  `number`  | The number of columns the panel uses.<br/> From 1 (minimum width) to 12 (full width). |

### Example json

```json
{
  "title": "Example",
  "rows": [
    {
      "title": "Per-job resource usage",
      "panels": [
        {
          "title": "Per-job CPU usage",
          "width": 6,
          "expr": [
            "sum(rate(process_cpu_seconds_total)) by (job)"
          ]
        },
        {
          "title": "Per-job RSS usage",
          "width": 6,
          "expr": [
            "sum(process_resident_memory_bytes) by (job)"
          ]
        },
        {
          "title": "Per-job disk read",
          "width": 6,
          "expr": [
            "sum(rate(process_io_storage_read_bytes_total)) by (job)"
          ]
        },
        {
          "title": "Per-job disk write",
          "width": 6,
          "expr": [
            "sum(rate(process_io_storage_written_bytes_total)) by (job)"
          ]
        }
      ]
    },
    {
      "title": "Free/used disk space",
      "panels": [
        {
          "unit": "MB",
          "expr": [
            "sum(vm_data_size_bytes{type!=\"indexdb\"}) / 1024 / 1024",
            "vm_free_disk_space_bytes / 1024 / 1024"
          ],
          "alias": [
            "usage space",
            "free space"
          ]
        }
      ]
    }
  ]
}
```

----


## App mode config options
vmui can be used to paste into other applications

1. Go to file `app/vmui/packages/vmui/public/index.html`
2. Find root element `<div id="root"></div>`
3. Add `data-params` with the options

### Options (JSON):

| Name                    |   Default   |                                                                           Description |
|:------------------------|:-----------:|--------------------------------------------------------------------------------------:|
| serverURL               | domain name |                                                          Can't be changed from the UI |
| useTenantID             |      -      |                           If the flag is present, the "Tenant ID" select is displayed |
| headerStyles.background |  `#FFFFFF`  |                                                               Header background color |
| headerStyles.color      |  `#3F51B5`  |                                                                     Header font color |
| palette.primary         |  `#3F51B5`  |                               used to represent primary interface elements for a user |
| palette.secondary       |  `#F50057`  |                             used to represent secondary interface elements for a user |
| palette.error           |  `#FF4141`  |            used to represent interface elements that the user should be made aware of |
| palette.warning         |  `#FF9800`  |                 used to represent potentially dangerous actions or important messages |
| palette.success         |  `#4CAF50`  |           used to indicate the successful completion of an action that user triggered |
| palette.info            |  `#03A9F4`  | used to present information to the user that is neutral and not necessarily important |

### JSON example:

```json
{
  "serverURL": "http://localhost:8428",
  "useTenantID": true,
  "headerStyles": {
    "background": "#FFFFFF",
    "color": "#538DE8"
  },
  "palette": {
    "primary": "#538DE8",
    "secondary": "#F76F8E",
    "error": "#FD151B",
    "warning": "#FFB30F",
    "success": "#7BE622",
    "info": "#0F5BFF"
  }
}
```

### HTML example:

```html
<div id="root" data-params='{"serverURL":"http://localhost:8428","useTenantID":true,"headerStyles":{"background":"#FFFFFF","color":"#538DE8"},"palette":{"primary":"#538DE8","secondary":"#F76F8E","error":"#FD151B","warning":"#FFB30F","success":"#7BE622","info":"#0F5BFF"}}'></div>
```

----

## Timezone configuration

vmui's timezone setting offers flexibility in displaying time data. It can be set through a configuration flag and is adjustable within the vmui interface. This feature caters to various user preferences and time zones.

### Default Timezone Setting

#### Via Configuration Flag

- Set the default timezone using the `--vmui.defaultTimezone` flag.
- Accepts a valid IANA Time Zone string (e.g., `America/New_York`, `Europe/Berlin`, `Etc/GMT+3`).
- If the flag is unset or invalid, vmui defaults to the browser's local timezone.

#### User Interface Adjustments

- Users can change the timezone in the vmui interface.
- Any changed setting in the interface overrides the flag's default, persisting for the user.
- The timezone specified in the `--vmui.defaultTimezone` flag is included in the vmui's timezone selection dropdown, aiding user choice.

### Key Points

- **Fallback to Browser's Local Timezone**: If the flag is not set or an invalid timezone is specified, vmui uses the local timezone of the user's browser.
- **User Preference Priority**: User-selected timezones in vmui take precedence over the default set by the flag.
- **Cluster Consistency**: Ensure uniform timezone settings across cluster nodes, but individual user interface selections will always override these defaults.

### Examples

Setting a default timezone, with user options to change:

```
./victoria-metrics --vmui.defaultTimezone="America/New_York"
```

In this scenario, if a user in Berlin accesses vmui without changing settings, it will default to their browser's local timezone (CET). If they select a different timezone in vmui, this choice will override the `"America/New_York"` setting for that user.
