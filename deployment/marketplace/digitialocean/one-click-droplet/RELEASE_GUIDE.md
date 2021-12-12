## Release guide for DigitalOcean 1-ClickApp Droplet

### Build image

Set preferable version of VictoriaMetrics e.g. `v1.70.0`:

```bash
export PKR_VAR_VM_VER='v1.70.0'
```

and run make:
```bash
make release-victoria-metrics-digitalocean-oneclick-droplet
```

to build the snapshot in DigitalOcean account.

### Update information on Vendor Portal

After packer build finished you need to update a product page.

1. Go to [https://cloud.digitalocean.com/vendorportal](https://cloud.digitalocean.com/vendorportal).
2. Choose a product that you need to update.
3. Enter a new information for this release and choose a droplet's snapshot which was built recently.
4. Submit updates for approve on DigitalOcean Marketplace.
