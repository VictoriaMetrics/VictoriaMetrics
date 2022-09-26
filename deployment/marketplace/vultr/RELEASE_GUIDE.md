## Release guide for Vultr Marketplace

### Build image

1. To build the snapshot in Vultr account you will need `VULTR_API_KEY` and [packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).
2. `VULTR_API_KEY` can be generated on [https://my.vultr.com/settings/#settingsapi](https://my.vultr.com/settings/#settingsapi) or use already generated from OnePassword.
3. Choose prefered version of VictoriaMetrics on [Github releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) page.
4. Set variables `VULTR_API_KEY` with `VM_VERSION` for `packer` environment and run make from example below:

```console
make release-victoria-metrics-vultr-server VULTR_API_KEY="your_token_here" VM_VERSION="prefered_release_version"
```
