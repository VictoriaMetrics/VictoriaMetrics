# Test cases

----

**Name:** Force execution of a queries

**Steps:**
1. click to button `Execute query`
2. click to icon `Refresh dashboard`
3. press `enter` on the query field

**Expected Result:**  
For each step sends a request and render new data

----

**Name:** Time Range with auto refresh

**Steps:**
1. Set absolute time range
2. Enable auto refresh
3. Change delay auto refresh
4. Disable auto refresh

**Expected Result:**  
Time range has not changed

----

**Name:** Query history

**Steps:**
1. Run query one by one: `1`, `2`, `3`
2. Press `Ctrl + ArrowUp`/`Ctrl + ArrowDown` when the query field focus

**Expected Result:**  
Query value changes according to execution order (Preserve execution order). 
<br/>
`Ctrl + ArrowUp` - set prev value, `Ctrl + ArrowDown` - set next value 

----

**Name:** Absolute time range fields

**Steps:**
1. Open `Time range controls`
2. Change `From` or `Until` time value
3. Click to `Apply`

**Expected Result:**  
When you change one of the fields, the second does not change

----

**Name:** Auto update after query delete

**Steps:**
1. Add multiple query
2. Execute queries
3. Delete one of the queries

**Expected Result:**  
Graph is automatically updated after the query delete

----

**Name:** Query URL params

**Steps:**
1. [Open graph](http://localhost:3000/?g0.range_input=1d&g0.end_input=2022-10-26T14%3A00%3A00&g0.step_input=180&g0.relative_time=none&g0.tab=chart&g0.expr=1&g1.range_input=1d&g1.end_input=2022-10-26T14%3A00%3A00&g1.step_input=180&g1.relative_time=none&g1.tab=chart&g1.expr=2#/) with params:
> ?g0.range_input=1d&g0.end_input=2022-10-26T14%3A00%3A00&g0.step_input=180&g0.relative_time=none&g0.tab=chart&g0.expr=1&g1.range_input=1d&g1.end_input=2022-10-26T14%3A00%3A00&g1.step_input=180&g1.relative_time=none&g1.tab=chart&g1.expr=2#/

**Expected Result:**  
Executed two query with params: 
```
query: 1 and 2
start: 1666706400
end: 1666792800
step: from "Step value" field (depends on screen width)
```
- Display two queries: `1` and `2`
- Time range from `2022-10-25 16:00:00` to `2022-10-26 16:00:00` (:warning: by UTC +2)
- Display tab `Table`

----

**Name:** Prometheus query URL params

**Steps:**
1. [Open graph](http://localhost:3000/?g0.expr=node_arp_entries&g0.tab=1&g0.stacked=0&g0.range_input=30m&g0.end_input=2021-09-11%2000%3A00%3A00&g0.moment_input=2021-09-11%2000%3A00%3A00&g0.step_input=6&g1.expr=node_cpu_guest_seconds_total&g1.tab=1&g1.stacked=0&g1.range_input=30m&g1.end_input=2022-12-01%2014%3A00%3A00&g1.moment_input=2022-12-01%2014%3A00%3A00&g1.step_input=6) with params:
> ?g0.expr=node_arp_entries&g0.tab=1&g0.stacked=0&g0.range_input=30m&g0.end_input=2021-09-11%2000%3A00%3A00&g0.moment_input=2021-09-11%2000%3A00%3A00&g0.step_input=6&g1.expr=node_cpu_guest_seconds_total&g1.tab=1&g1.stacked=0&g1.range_input=30m&g1.end_input=2022-12-01%2014%3A00%3A00&g1.moment_input=2022-12-01%2014%3A00%3A00&g1.step_input=6

**Expected Result:**
- Display two queries: `node_arp_entries` and `node_cpu_guest_seconds_total`
- Time range from `2021-09-11 01:30:00` to `2021-09-11 02:00:00` (:warning: by UTC +2)
- Display tab `Table`

----
