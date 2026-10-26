locals {
  name = var.deployment_name

  dns_domain_name  = trimsuffix(var.dns_domain_name, ".")
  ami_architecture = { amd64 = "x86_64", arm64 = "arm64" }[var.yact_architecture]

  yact_linux_binary_path = pathexpand(var.yact_linux_binary_path)
  yact_binary_sha256     = filesha256(local.yact_linux_binary_path)

  yact_bin_dir      = "/usr/local/bin"
  yact_config       = "/etc/yact/config.yml"
  yact_cert_dir     = "/etc/yact/certs"
  yact_server_certs = "${local.yact_cert_dir}/server"
  yact_service_path = "/etc/systemd/system/yact.service"
  yact_service_name = "yact"
  yact_control_port = tonumber(split(":", var.yact_proxy_addr)[1])

  # Certificates are generated on the laptop with `yact make-certs` and the
  # server set is uploaded to the instance. The default is yact's own cert
  # location, so the laptop client picks it up without an explicit cert_dir.
  local_cert_dir = pathexpand(var.yact_local_cert_dir)

  # Laptop client config, written to disk by a local_file resource. cert_dir is
  # omitted so yact resolves its own default (~/.config/yact/certs/client1),
  # unless the certificates were moved somewhere non-standard.
  yact_local_config_path = pathexpand(var.yact_local_config_path)

  yact_client_config = merge(
    {
      proxy_addr = "${aws_instance.yact.public_ip}:${local.yact_control_port}"
    },
    local.local_cert_dir == pathexpand("~/.config/yact/certs") ? {} : {
      tls_config = {
        cert_dir = "${local.local_cert_dir}/client1"
      }
    },
    {
      to_addrs = [
        for port in var.public_service_ports : {
          bind_address = ":${port}"
          local_port   = port
        }
      ]
    },
  )

  yact_cert_domains = [
    "${var.dns_record_name}.${local.dns_domain_name}",
    "*.${var.dns_record_name}.${local.dns_domain_name}",
  ]

  public_listener_ports = toset([for port in var.public_service_ports : tostring(port)])

  public_ingress_rules = {
    for rule in setproduct(local.public_listener_ports, ["0.0.0.0/0"]) : "${rule[0]}-${rule[1]}" => {
      port = tonumber(rule[0])
      cidr = rule[1]
    }
  }

  laptop_listener_ports = toset(["22", tostring(local.yact_control_port)])

  laptop_ingress_rules = {
    for rule in setproduct(local.laptop_listener_ports, var.laptop_cidr_blocks) : "${rule[0]}-${rule[1]}" => {
      port = tonumber(rule[0])
      cidr = rule[1]
    }
  }

  tags = merge(var.tags, {
    Name       = var.deployment_name
    Deployment = var.deployment_name
  })
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-${local.ami_architecture}"]
  }

  filter {
    name   = "architecture"
    values = [local.ami_architecture]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_route53_zone" "domain" {
  name         = "${local.dns_domain_name}."
  private_zone = false
}

resource "aws_resourcegroups_group" "rg" {
  name = "${local.name}-rg"

  resource_query {
    type = "TAG_FILTERS_1_0"

    query = jsonencode({
      ResourceTypeFilters = ["AWS::AllSupported"]
      TagFilters = [
        {
          Key    = "Deployment"
          Values = [var.deployment_name]
        }
      ]
    })
  }

  tags = local.tags
}

resource "aws_vpc" "vpc" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.tags, {
    Name = "${local.name}-vpc"
  })
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.vpc.id

  tags = merge(local.tags, {
    Name = "${local.name}-igw"
  })
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.vpc.id
  cidr_block              = var.public_subnet_cidr
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = merge(local.tags, {
    Name = "${local.name}-public-subnet"
  })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }

  tags = merge(local.tags, {
    Name = "${local.name}-public-rt"
  })
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "instance" {
  name        = "${local.name}-instance-sg"
  description = "Allow public yact traffic and laptop-only control access"
  vpc_id      = aws_vpc.vpc.id

  tags = merge(local.tags, {
    Name = "${local.name}-instance-sg"
  })
}

resource "aws_vpc_security_group_ingress_rule" "public" {
  for_each = local.public_ingress_rules

  security_group_id = aws_security_group.instance.id
  cidr_ipv4         = each.value.cidr
  from_port         = each.value.port
  ip_protocol       = "tcp"
  to_port           = each.value.port
  tags              = local.tags
}

resource "aws_vpc_security_group_ingress_rule" "laptop" {
  for_each = local.laptop_ingress_rules

  security_group_id = aws_security_group.instance.id
  cidr_ipv4         = each.value.cidr
  from_port         = each.value.port
  ip_protocol       = "tcp"
  to_port           = each.value.port
  tags              = local.tags
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.instance.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  tags              = local.tags
}

resource "aws_key_pair" "ssh" {
  key_name   = "${local.name}-ssh"
  public_key = file(pathexpand(var.ssh_public_key_path))

  tags = merge(local.tags, {
    Name = "${local.name}-ssh"
  })
}

resource "aws_instance" "yact" {
  ami                         = data.aws_ami.amazon_linux.id
  instance_type               = var.instance_type
  key_name                    = aws_key_pair.ssh.key_name
  subnet_id                   = aws_subnet.public.id
  vpc_security_group_ids      = [aws_security_group.instance.id]
  associate_public_ip_address = true
  user_data_replace_on_change = true

  # The binary checksum is part of user_data so that a new yact build forces a
  # new instance to be created and provisioned.
  user_data = templatefile("${path.module}/cloud-init.yaml", {
    yact_binary_sha256   = local.yact_binary_sha256
    yact_config_path     = local.yact_config
    yact_cert_dir        = local.yact_cert_dir
    yact_bin_dir         = local.yact_bin_dir
    yact_service_path    = local.yact_service_path
    yact_service_name    = local.yact_service_name
    yact_proxy_addr      = var.yact_proxy_addr
    yact_server_cert_dir = local.yact_server_certs
  })

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = "gp3"
  }

  volume_tags = merge(local.tags, {
    Name = "${local.name}-yact-root"
  })

  tags = merge(local.tags, {
    Name = "${local.name}-yact"
  })
}

resource "aws_route53_record" "host" {
  zone_id         = data.aws_route53_zone.domain.zone_id
  name            = "${var.dns_record_name}.${local.dns_domain_name}"
  type            = "A"
  ttl             = var.dns_ttl
  records         = [aws_instance.yact.public_ip]
  allow_overwrite = true
}

resource "aws_route53_record" "wildcard" {
  zone_id         = data.aws_route53_zone.domain.zone_id
  name            = "*.${var.dns_record_name}.${local.dns_domain_name}"
  type            = "A"
  ttl             = var.dns_ttl
  records         = [aws_instance.yact.public_ip]
  allow_overwrite = true
}
