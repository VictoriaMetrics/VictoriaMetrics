---
sort: 19
---

# VictoriaMetrics best practices


## Install Recommendation

It is recommended running the latest available release of VictoriaMetrics from [this page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases), since it contains all the bugfixes and enhancements.

There is no need to tune VictoriaMetrics because it uses reasonable defaults for command-line flags.  These flags are automatically adjusted for the available CPU and RAM resources. There is no need in Operating System tuning because VictoriaMetrics is optimized for default OS settings. The only option is to increase the limit on the [number of open files in the OS](https://medium.com/@muhammadtriwibowo/set-permanently-ulimit-n-open-files-in-ubuntu-4d61064429a), so VictoriaMetrics could accept more incoming connections and could keep open more data files.


## Filesystem

The recommended filesystem for VictoriaMetrics is [ext4](https://en.wikipedia.org/wiki/Ext4). If you plan to store more than 1TB of data on ext4 partition or plan to extend it to more than 16TB, then the following options are recommended to pass to mkfs.ext4:

```
mkfs.ext4 ... -O 64bit,huge_file,extent -T huge
```

VictoriaMetrics should work OK with other filesystems, including network filesystems such as [NFS](https://en.wikipedia.org/wiki/Network_File_System), [Amazon EFS](https://aws.amazon.com/efs/) and [Google Filestore](https://cloud.google.com/filestore).


## Operation System

VictoriaMetrics is production-ready for the following operating systems:

* Linux (Alpine, Ubuntu, Debian, RedHat, etc.)
* FreeBSD
* OpenBSD

Some VictoriaMetrics components ([vmagent](https://docs.victoriametrics.com/vmagent.html), [vmalert](https://docs.victoriametrics.com/vmalert.html) and [vmauth](https://docs.victoriametrics.com/vmauth.html)) can run on Windows.

VictoriaMetrics can run also on MacOS for testing and development purposes.


## Upgrade procedure

It is safe to upgrade VictoriaMetrics to new versions unless the [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) say otherwise. It is safe to skip multiple versions during the upgrade unless release notes say otherwise. It is recommended to perform regular upgrades to the latest version, since it may contain important bug fixes, performance optimizations or new features.

It is also safe to downgrade to the previous version unless release notes say otherwise.

The following steps must be performed during the upgrade / downgrade procedure:

* Send SIGINT signal to VictoriaMetrics process so that it is stopped gracefully.
* Wait until the process stops. This can take a few seconds.
* Start the upgraded VictoriaMetrics.


## Backup Recommendations

VictoriaMetrics supports backups via [vmbackup](https://docs.victoriametrics.com/vmbackup.html) and [vmrestore](https://docs.victoriametrics.com/vmrestore.html) tools. There is also [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager.html), which simplifies backup automation.


## Technical Support and Services

There are the following channels for providing technical support for VictoriaMetrics:

* [Github issues](https://github.com/VictoriaMetrics/VictoriaMetrics/issues)
* [Slack channel](https://slack.victoriametrics.com/)
* [Telegram channel](https://t.me/VictoriaMetrics_en)

We also provide [Enterprise support](https://victoriametrics.com/products/enterprise/).
