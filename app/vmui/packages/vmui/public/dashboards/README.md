### Setup
1. Create `.json` config file in a folder `dashboards`
2. Import your config file into the `dashboards/index.js`
3. Add filename into the array `window.__VMUI_PREDEFINED_DASHBOARDS__`

### Configuration options

<br/>
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

---

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
