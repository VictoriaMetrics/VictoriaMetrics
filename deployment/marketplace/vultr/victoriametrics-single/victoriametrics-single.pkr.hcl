variable "vultr_api_key" {
  type      = string
  default   = "${env("VULTR_API_KEY")}"
  sensitive = true
}

variable "victoriametrics_version" {
  type        = string
  default     = "${env("VM_VERSION")}"
  description = "Version number of the desired VictoriaMetrics binary."
}

packer {
    required_plugins {
        vultr = {
            version = ">=v2.3.2"
            source = "github.com/vultr/vultr"
        }
    }
}

source "vultr" "victoriametrics-single" {
  api_key              = "${var.vultr_api_key}"
  os_id                = "387"
  plan_id              = "vc2-1c-1gb"
  region_id            = "ewr"
  snapshot_description = "victoriametrics-snapshot-${formatdate("YYYY-MM-DD hh:mm", timestamp())}"
  ssh_username         = "root"
  state_timeout        = "10m"
}

build {
  sources = ["source.vultr.victoriametrics-single"]

  provisioner "file" {
    source = "helper-scripts/vultr-helper.sh"
    destination = "/root/vultr-helper.sh"
  }

  provisioner "file" {
    source = "victoriametrics-single/setup-per-boot.sh"
    destination = "/root/setup-per-boot.sh"
  }

  # Copy configuration files
  provisioner "file" {
    destination = "/etc/"
    source      = "victoriametrics-single/etc/"
  }

  provisioner "file" {
    source = "victoriametrics-single/setup-per-instance.sh"
    destination = "/root/setup-per-instance.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "VM_VERSION=${var.victoriametrics_version}",
      "DEBIAN_FRONTEND=noninteractive"
    ]
      script = "victoriametrics-single/victoriametrics-single.sh"
      remote_folder = "/root"
      remote_file = "victoriametrics-single.sh"
  }
}
