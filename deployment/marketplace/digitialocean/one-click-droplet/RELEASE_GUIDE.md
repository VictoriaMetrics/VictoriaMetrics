## Release guide for DigitalOcean 1-ClickApp Droplet

### Build image

To build the snapshot in DigitalOcean account you will need API Token and [packer](https://learn.hashicorp.com/tutorials/packer/get-started-install-cli).

API Token can be generated on [https://cloud.digitalocean.com/account/api/tokens](https://cloud.digitalocean.com/account/api/tokens) or use already generated from OnePassword.

Set variable `DIGITALOCEAN_API_TOKEN` for environment:

```bash
export DIGITALOCEAN_API_TOKEN="your_token_here"
```

or set it by with make:

```bash
make release-victoria-metrics-digitalocean-oneclick-droplet DIGITALOCEAN_API_TOKEN="your_token_here"
```

### Update information on Vendor Portal

After packer build finished you need to update a product page.

1. Go to [https://cloud.digitalocean.com/vendorportal](https://cloud.digitalocean.com/vendorportal).
2. Choose a product that you need to update.
3. Enter newer information for this release and choose a droplet's snapshot which was builded recently.
4. Submit updates for approve on DigitalOcean Marketplace.
