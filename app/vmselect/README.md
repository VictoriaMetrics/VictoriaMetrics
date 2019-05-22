`vmselect` performs the following tasks:

- Splits incoming selects to tasks for `vmstorage` nodes and issues these tasks
  to all the `vmstorage` nodes in the cluster.

- Merges responses from all the `vmstorage` nodes and returns a single response.
