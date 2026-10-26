# AWS yact

Terraform deployment for a [`yact`](https://github.com/snobb/go-yact) proxy server on AWS.

The module creates:

- `endpoint` and `*.endpoint` Route53 A records pointing at the EC2 public IP.
- Public ingress for the ports in `public_service_ports` (default `443`).
- Laptop-only ingress for SSH and the yact control port `8001`.
- A locally built yact linux binary uploaded to `/usr/local/bin/yact`.
- An mTLS server config in `/etc/yact/config.yml` and a `yact.service` systemd
  unit enabled at boot.

Certificates are generated on the laptop with `yact make-certs`; only the server
set is uploaded to the instance. The single client certificate stays local.

## Prerequisites

`yact` must be on your `PATH` — Terraform shells out to `yact make-certs` to
generate certificates.

Build the linux binary before applying. Cross-compile to a separate name so the
laptop build in `bin/yact` is preserved:

```sh
cd ~/code/go-yact.git
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/yact_linux_arm64 ./cmd
```

Use `GOARCH=amd64` for `t3.*` instance types. Do not use `make build-linux`: it
depends on `clean`, which does `rm -rf ./bin` and would delete the macOS binary
the laptop client uses.

The binary path is set in `terraform.tfvars` via `yact_linux_binary_path`. It
must be a linux build — uploading a macOS binary passes plan (only its hash is
read) but fails at apply.

## Configuration

```hcl
aws_region          = "eu-west-2"
aws_profile         = "your-profile-name"
laptop_cidr_blocks  = ["203.0.113.10/32"]
yact_linux_binary_path = "~/code/go-yact.git/bin/yact_linux_arm64"
```

`yact_architecture` selects the Amazon Linux AMI architecture. Keep
`instance_type` compatible with it: `t3.nano` for `amd64`, `t4g.nano` for
`arm64`.

The instance public IP is always embedded in the server certificate as a SAN.
Certificates are generated after the instance is created so the IP is known. Add
`yact_cert_ip_addresses` for any extra addresses.

## Usage

Certificates are generated in `~/.config/yact/certs` (override with
`yact_local_cert_dir`). Only the `server/` set is uploaded to the instance; the
`client1/` set stays on the laptop.

Apply, then check the service:

```sh
terraform output yact_server_log_command   # ssh + sudo journalctl -u yact -f
```

yact runs as a systemd unit and is enabled at boot. Terraform starts it once the
binary and certificates are uploaded, so no manual start or reboot is needed.

Apply also writes the laptop client config to
`~/.config/yact/config.yml` (override with `yact_local_config_path`). Print and
run the client command:

```sh
terraform output -raw yact_client_command
```

Then run the printed command yourself. Prefer this over piping the output into
`sh`: the config contains a host and ports taken from variables, and reading
what you are about to execute is cheap.

yact reads `~/.config/yact/config.yml` by default. When
`yact_local_config_path` is overridden, the generated command passes `-c` with
the custom path.

The generated config omits `cert_dir` when the certificates are in yact's default
location, and sets it explicitly otherwise.

The client must stay running: the proxy only binds a port while a client is
connected, and it tears the listener down when the client disconnects. Something
also needs to be listening on the client side's `local_port` for traffic to be
forwarded.

Ports below 1024 work because the unit runs as root.

## Updating the binary

The binary checksum is embedded in the instance user data, and
`user_data_replace_on_change` is set, so changing `yact_linux_binary_path` or
its contents causes Terraform to replace the instance and re-upload. Check
`yact_binary_sha256` to confirm which build is deployed, and compare it with
`shasum -a 256 ~/code/go-yact.git/bin/yact_linux_arm64`.

Replacing the instance changes its public IP. That regenerates the
certificates, and with them the CA — see below.

## Regenerating certificates

Certificates live in `~/.config/yact/certs` unless `yact_local_cert_dir` says
otherwise. Changing `yact_cert_ip_addresses`, the DNS names, or the instance
public IP regenerates them and re-uploads the `server/` set. The `client1/` set
stays on the laptop.

To force regeneration and re-upload without changing those inputs, run:

```sh
terraform apply -replace=terraform_data.yact_certs
```

Deleting local certificates alone does not trigger Terraform to regenerate
them. If a previous version of this deployment generated multiple clients,
remove any unused `client2/`, `client3/`, etc. directories yourself; the current
deployment only generates `client1/`.

**Regenerating issues a new CA.** Any laptop client holding a certificate from
a previous run will be rejected by the server until it picks up the new
`client1` set. `yact make-certs` always mints a fresh CA, so there is no way to
avoid this from Terraform alone.
