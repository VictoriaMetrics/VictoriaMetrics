variable "token" {
  type        = string
  default     = "${env("DIGITALOCEAN_API_TOKEN")}"
  description = "DigitalOcean API token used to create droplets."
}

variable "image_id" {
  type        = string
  default     = "ubuntu-20-04-x64"
  description = "DigitalOcean linux image ID."
}

variable "victoriametrics_version" {
  type        = string
  default     = "${env("VM_VERSION")}"
  description = "Version number of the desired VictoriaMetrics binary."
}

variable "image_name" {
  type        = string
  default     = "victoriametrics-snapshot-{{timestamp}}"
  description = "Name of the snapshot created on DigitalOcean."
}

source "digitalocean" "default" {
  api_token     = "${var.token}"
  image         = "${var.image_id}"
  region        = "nyc3"
  size          = "s-1vcpu-1gb"
  snapshot_name = "${var.image_name}"
  ssh_username  = "root"
}

build {
  sources = ["source.digitalocean.default"]

  provisioner "file" {
    destination = "/etc/"
    source      = "files/etc/"
  }

  provisioner "file" {
    destination = "/var/"
    source      = "files/var/"
  }

  # Setup instance configuration
  provisioner "shell" {
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive"
    ]
    scripts = [
      "scripts/01-setup.sh",
      "scripts/02-firewall.sh",
    ]
  }

  # Install VictoriaMetrics
  provisioner "shell" {
    environment_vars = [
      "VM_VERSION=${var.victoriametrics_version}",
      "DEBIAN_FRONTEND=noninteractive"
    ]
    scripts = [
      "scripts/04-install-victoriametrics.sh",
    ]
  }

  # Cleanup and validate instance
  provisioner "shell" {
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive"
    ]
    scripts = [
      "scripts/89-cleanup-logs.sh",
      "scripts/90-cleanup.sh",
      "scripts/99-img-check.sh"
    ]
  }
}
