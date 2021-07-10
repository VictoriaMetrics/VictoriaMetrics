`vmselect` performs the following tasks:

- Splits incoming selects to tasks for `vmstorage` nodes and issues these tasks
  to all the `vmstorage` nodes in the cluster.

- Merges responses from all the `vmstorage` nodes and returns a single response.

The `vmui` directory contains static contents built from [app/vmui](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmui) package with `make vmui-update` command. The `vmui` page is available at `http://<vmselect>:8481/select/<accountID>/vmui/`.
