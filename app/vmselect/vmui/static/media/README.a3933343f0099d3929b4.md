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

| Name           |    Type    |                                         Description |
|:---------------|:----------:|----------------------------------------------------:|
| expr*          | `string[]` |                                 Data source queries |
| title          |  `string`  |                                         Panel title |
| description    |  `string`  |              Additional information about the panel |
| unit           |  `string`  |                                         Y-axis unit |
| showLegend     | `boolean`  | If `false`, the legend hide. Default value - `true` |

---

### Example json

```json
{
  "title": "Example",
  "rows": [
    {
      "title": "Performance",
      "panels": [
        {
          "title": "Query duration",
          "description": "The less time it takes is better.\n* `*` - unsupported query path\n* `/write` - insert into VM\n* `/metrics` - query VM system metrics\n* `/query` - query instant values\n* `/query_range` - query over a range of time\n* `/series` - match a certain label set\n* `/label/{}/values` - query a list of label values (variables mostly)",
          "unit": "ms",
          "showLegend": false,
          "expr": [
            "max(vm_request_duration_seconds{quantile=~\"(0.5|0.99)\"}) by (path, quantile) > 0"
          ]
        },
        {
          "title": "Concurrent flushes on disk",
          "description": "Shows how many ongoing insertions (not API /write calls) on disk are taking place, where:\n* `max` - equal to number of CPUs;\n* `current` - current number of goroutines busy with inserting rows into underlying storage.\n\nEvery successful API /write call results into flush on disk. However, these two actions are separated and controlled via different concurrency limiters. The `max` on this panel can't be changed and always equal to number of CPUs. \n\nWhen `current` hits `max` constantly, it means storage is overloaded and requires more CPU.\n\n",
          "expr": [
            "sum(vm_concurrent_addrows_capacity)",
            "sum(vm_concurrent_addrows_current)"
          ]
        }
      ]
    },
    {
      "title": "Troubleshooting",
      "panels": [
        {
          "title": "Churn rate",
          "description": "Shows the rate and total number of new series created over last 24h.\n\nHigh churn rate tightly connected with database performance and may result in unexpected OOM's or slow queries. It is recommended to always keep an eye on this metric to avoid unexpected cardinality \"explosions\".\n\nThe higher churn rate is, the more resources required to handle it. Consider to keep the churn rate as low as possible.\n\nGood references to read:\n* https://www.robustperception.io/cardinality-is-key\n* https://www.robustperception.io/using-tsdb-analyze-to-investigate-churn-and-cardinality",
          "expr": [
            "sum(rate(vm_new_timeseries_created_total[5m]))",
            "sum(increase(vm_new_timeseries_created_total[24h]))"
          ]
        }
      ]
    }
  ]
}
```