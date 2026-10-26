variable "deployment_name" {
  type        = string
  description = "Prefix for all resource names"
}

variable "aws_region" {
  type        = string
  description = "AWS region"
}

variable "aws_profile" {
  type        = string
  description = "AWS shared config/credentials profile to use"
  default     = null
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR block for the VPC"
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  type        = string
  description = "CIDR block for the public subnet"
  default     = "10.0.1.0/24"
}

variable "laptop_cidr_blocks" {
  type        = list(string)
  description = "Laptop public CIDR blocks allowed to reach SSH and the yact control port"
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type for the yact proxy server"
  default     = "t4g.nano"
}

# Keep instance_type architecture-compatible with yact_architecture:
# t3.* and similar types are amd64; t4g.* and similar Graviton types are arm64.
variable "yact_architecture" {
  type        = string
  description = "CPU architecture of the yact linux binary to upload (must match the AMI architecture)"
  default     = "arm64"

  validation {
    condition     = contains(["amd64", "arm64"], var.yact_architecture)
    error_message = "yact_architecture must be either amd64 or arm64."
  }
}

variable "yact_linux_binary_path" {
  type        = string
  description = "Path to the locally built yact linux binary to upload to the instance (see 'make yact-linux' in README)"
}

variable "root_volume_size" {
  type        = number
  description = "Root EBS volume size in GiB"
  default     = 8
}

variable "ssh_public_key_path" {
  type        = string
  description = "Path to the SSH public key to import for EC2 access"
  default     = "~/.ssh/id_ed25519.pub"
}

variable "yact_proxy_addr" {
  type        = string
  description = "Address the yact proxy server listens on for clients. Must not be loopback-only, laptops connect over the public IP."
  default     = "0.0.0.0:8001"
}

variable "public_service_ports" {
  type        = list(number)
  description = "Public ports the proxy server listens on for tunnelled traffic"
  default     = [443]
}

variable "yact_local_cert_dir" {
  type        = string
  description = "Local directory for the yact certificates generated on the laptop. Defaults to yact's own location, so the client needs no explicit cert_dir."
  default     = "~/.config/yact/certs"
}

variable "yact_local_config_path" {
  type        = string
  description = "Path the laptop client config is written to. yact resolves its default config as ./config.yml, so the client is invoked with an explicit -c pointing here."
  default     = "~/.config/yact/config.yml"
}

variable "yact_cert_ip_addresses" {
  type        = list(string)
  description = "Extra IP addresses to embed in the yact server certificate (SANs). The instance public IP is always included; set this for additional addresses."
  default     = []
}

variable "dns_domain_name" {
  type        = string
  description = "Route53 public hosted zone name"
}

variable "dns_record_name" {
  type        = string
  description = "DNS record prefix to create under dns_domain_name"
}

variable "dns_ttl" {
  type        = number
  description = "TTL for Route53 A records"
  default     = 60
}

variable "tags" {
  type        = map(string)
  description = "Additional tags to apply to resources (for example a creator tag)"
  default     = {}
}
