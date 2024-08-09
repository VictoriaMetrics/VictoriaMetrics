---
weight: 6
title: Backup and Restore
menu:
  docs:
    identifier: "victorialogs-backup-and-restore"
    parent: "victorialogs"
    weight: 6
    title: Backup
aliases:
- /VictoriaLogs/backup-and-restore.html
---

## Backup
VictoriaLogs currently does not have a snapshot feature and a tool like vmbackup as VictoriaMetrics does. 
So backing up VictoriaLogs requires manually executing the `rsync` command. 

The files in VictoriaLogs have the following properties:
- All the data files are immutable. Small metadata files can be modified.
- Old data files are periodically merged into new data files.

Therefore, for a complete data backup, you need to run the `rsync` command twice.

```bash
# example of rsync to remote host
rsync -avh --progress <path-to-victorialogs-data> <username>@<host>:<path-to-victorialogs-backup>
```

The first `rsync` will sync the majority of the data, which can be time-consuming. 
As VictoriaLogs continues to run, new data is ingested, potentially creating new data files and modifying metadata files.

```bash
# example output
sending incremental file list
victoria-logs-data/
victoria-logs-data/flock.lock
              0 100%    0.00kB/s    0:00:00 (xfr#1, to-chk=78/80)
              
...

victoria-logs-data/partitions/20240809/indexdb/17E9ED7EF89BF422/metaindex.bin
             51 100%    5.53kB/s    0:00:00 (xfr#64, to-chk=0/80)

sent 12.19K bytes  received 1.30K bytes  3.86K bytes/sec
total size is 7.31K  speedup is 0.54
```

The second `rsync` **requires a brief shutdown of VictoriaLogs** to ensure all data and metadata files are consistent and no longer changing. 
This `rsync` will cover any changes that have occurred since the last `rsync` and should not take a significant amount of time.

## Restore
To restore from a backup, simply `rsync` the backup files from a remote location to the original directory during downtime. 
VictoriaLogs will automatically load this data upon startup.

```bash
# example of rsync from remote backup to local
rsync -avh --progress <username>@<host>:<path-to-victorialogs-backup> <path-to-victorialogs-data>
```