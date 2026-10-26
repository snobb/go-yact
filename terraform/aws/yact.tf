# mTLS certificates are generated on the laptop and the server set is uploaded
# to the instance. Regenerated whenever the domain list or SAN IPs change.

locals {
  # The instance public IP is unknown until the instance is created, so it can
  # only be embedded in the certificate if certificates are generated after it.
  yact_cert_ip_addresses = distinct(concat(
    [aws_instance.yact.public_ip],
    var.yact_cert_ip_addresses,
  ))

  yact_make_certs_command = join(" ", concat(
    ["yact", "make-certs", "-R", local.local_cert_dir],
    flatten([for d in local.yact_cert_domains : ["-d", d]]),
    flatten([for ip in local.yact_cert_ip_addresses : ["-a", ip]]),
    ["-n", "1"],
  ))

  yact_ssh_private_key = pathexpand(trimsuffix(var.ssh_public_key_path, ".pub"))

  yact_install_script = <<-EOT
    set -eu

    sudo install -m 0755 -o root -g root /tmp/yact ${local.yact_bin_dir}/yact
    sudo install -m 0644 -o root -g root /tmp/ca.crt ${local.yact_server_certs}/ca.crt
    sudo install -m 0644 -o root -g root /tmp/server.crt ${local.yact_server_certs}/server.crt
    sudo install -m 0600 -o root -g root /tmp/server.key ${local.yact_server_certs}/server.key

    # Sanity check that the uploaded binary is executable on this instance.
    # The unit runs as root, so it can bind ports below 1024 without a file
    # capability on the binary.
    sudo ${local.yact_bin_dir}/yact -h >/dev/null

    # The unit was enabled at boot but not started, because the binary and
    # certificates only exist now. Start it so the proxy runs without a reboot.
    sudo systemctl daemon-reload
    sudo systemctl restart ${local.yact_service_name}

    rm -f /tmp/yact /tmp/ca.crt /tmp/server.crt /tmp/server.key
  EOT
}

resource "terraform_data" "yact_certs" {
  # The instance is awaited so its public IP is known and can be embedded as a
  # SAN in the generated certificate.
  depends_on = [aws_instance.yact]

  triggers_replace = {
    domains      = join(",", local.yact_cert_domains)
    ip_addresses = join(",", local.yact_cert_ip_addresses)
  }

  provisioner "local-exec" {
    command = local.yact_make_certs_command
  }
}

# Write the laptop client config. yact reads ~/.config/yact/config.yml by
# default; the generated command passes -c only for a custom path.
resource "local_file" "yact_client_config" {
  filename        = local.yact_local_config_path
  content         = yamlencode(local.yact_client_config)
  file_permission = "0600"
}

# SSH is not immediately available once the instance reports as running, so
# poll until the key based login succeeds before uploading anything.

resource "terraform_data" "yact_wait_ssh" {
  triggers_replace = {
    instance_id = aws_instance.yact.id
  }

  provisioner "local-exec" {
    command = <<-EOT
      set -eu
      for _ in $(seq 1 60); do
        if ssh -i ${local.yact_ssh_private_key} \
          -o StrictHostKeyChecking=no \
          -o UserKnownHostsFile=/dev/null \
          -o BatchMode=yes \
          -o ConnectTimeout=5 \
          ec2-user@${aws_instance.yact.public_ip} true; then
          exit 0
        fi
        sleep 5
      done

      echo "ssh to ${aws_instance.yact.public_ip} did not become ready" >&2
      exit 1
    EOT
  }
}

resource "terraform_data" "yact_upload" {
  triggers_replace = {
    instance_id = aws_instance.yact.id
    certs       = terraform_data.yact_certs.id
  }

  depends_on = [
    terraform_data.yact_certs,
    terraform_data.yact_wait_ssh,
  ]

  connection {
    type        = "ssh"
    user        = "ec2-user"
    host        = aws_instance.yact.public_ip
    private_key = file(local.yact_ssh_private_key)
  }

  provisioner "file" {
    source      = local.yact_linux_binary_path
    destination = "/tmp/yact"
  }

  provisioner "file" {
    source      = "${local.local_cert_dir}/server/ca.crt"
    destination = "/tmp/ca.crt"
  }

  provisioner "file" {
    source      = "${local.local_cert_dir}/server/server.crt"
    destination = "/tmp/server.crt"
  }

  provisioner "file" {
    source      = "${local.local_cert_dir}/server/server.key"
    destination = "/tmp/server.key"
  }

  provisioner "remote-exec" {
    inline = [local.yact_install_script]
  }
}
