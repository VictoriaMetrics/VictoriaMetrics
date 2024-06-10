## Release guide for DigitalOcean 1-ClickApp Droplet

### Build image

1. To build the snapshot in DigitalOcean account you will need API Token and [packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).
2. API Token can be generated on [https://cloud.digitalocean.com/account/api/tokens](https://cloud.digitalocean.com/account/api/tokens) or use already generated from OnePassword.
3. Choose prefered version of VictoriaMetrics on [Github releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) page.
4. Set variables `DIGITALOCEAN_API_TOKEN` with `VM_VERSION` for `packer` environment and run make from example below:

```console
make release-victoria-metrics-digitalocean-oneclick-droplet DIGITALOCEAN_API_TOKEN="dop_v23_2e46f4759ceeeba0d0248" VM_VERSION="1.94.0"
```


## Release guide for DigitalOcean Kubernetes 1-Click App

### Submit a pull request

1. Fork [https://github.com/digitalocean/marketplace-kubernetes](https://github.com/digitalocean/marketplace-kubernetes).
2. Apply changes to vmagent.yaml and vmcluster.yaml in https://github.com/digitalocean/marketplace-kubernetes/tree/master/stacks/victoria-metrics-cluster/yaml .
3. Send a PR to https://github.com/digitalocean/marketplace-kubernetes.
4. Add changes to product page at [https://cloud.digitalocean.com/vendorportal/61de9e7fbbd94c7e4b9b80be/15/edit](https://cloud.digitalocean.com/vendorportal/61de9e7fbbd94c7e4b9b80be/15/edit):
 * update App Version;
 * (onfly if PR was submittedm apprived and merged) add select a checkbox "I made a change, submitted a pull request, and the pull request was approved and merged."
 * updated Version of packages and links to changelogs in `Software Included` section;
 * describe your updates in `Reason for update` section.
 * submit your changes.


### Update information on Vendor Portal


After packer build finished you need to update a product page.

1. Go to [https://cloud.digitalocean.com/vendorportal](https://cloud.digitalocean.com/vendorportal).
2. Choose a product that you need to update.
3. Enter newer information for this release and choose a droplet's snapshot which was builded recently.
4. Submit updates for approve on DigitalOcean Marketplace.
