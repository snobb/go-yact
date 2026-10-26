output "resource_group_name" {
  description = "AWS Resource Group containing resources tagged for this deployment."
  value       = aws_resourcegroups_group.rg.name
}

output "instance_id" {
  description = "ID of the EC2 instance running the yact proxy."
  value       = aws_instance.yact.id
}

output "instance_public_ip" {
  description = "Current public IP of the EC2 instance."
  value       = aws_instance.yact.public_ip
}

output "ssh_command" {
  description = "SSH command for the EC2 instance."
  value       = "ssh -i ${local.yact_ssh_private_key} ec2-user@${aws_instance.yact.public_ip}"
}

output "dns_record" {
  description = "Route53 record for the deployment."
  value       = aws_route53_record.host.fqdn
}

output "wildcard_dns_record" {
  description = "Wildcard Route53 record for the deployment."
  value       = aws_route53_record.wildcard.fqdn
}

output "domain_url" {
  description = "HTTPS URL for the primary endpoint."
  value       = "https://${aws_route53_record.host.fqdn}"
}

output "yact_server_command" {
  description = "SSH command to restart the yact service on the EC2 instance."
  value       = "ssh -t -i ${local.yact_ssh_private_key} ec2-user@${aws_instance.yact.public_ip} 'sudo systemctl restart ${local.yact_service_name}'"
}

output "yact_server_log_command" {
  description = "SSH command to follow the yact proxy log on the EC2 instance. The unit logs to the journal, not a file."
  value       = "ssh -t -i ${local.yact_ssh_private_key} ec2-user@${aws_instance.yact.public_ip} 'sudo journalctl -u ${local.yact_service_name} -f'"
}

output "yact_server_cert_dir" {
  description = "Local directory holding the uploaded yact server certificate set."
  value       = abspath("${local.local_cert_dir}/server")
}

output "yact_client_cert_dir" {
  description = "Local directory holding the yact client certificate to use on the laptop."
  value       = abspath("${local.local_cert_dir}/client1")
}

output "yact_client_config_path" {
  description = "Path the laptop client config is written to. Run the client with -c pointing at this file."
  value       = local.yact_local_config_path
}

output "yact_client_command" {
  description = "Command to run the yact client on the laptop, using the generated config."
  value = join(" ", concat(
    ["yact"],
    local.yact_local_config_path == pathexpand("~/.config/yact/config.yml") ? [] : ["-c", local.yact_local_config_path],
    ["client"],
    flatten([for port in var.public_service_ports : ["-L", ":${port}:${port}"]]),
  ))
}

output "yact_binary_sha256" {
  description = "Hex SHA256 of the uploaded yact linux binary, comparable with `shasum -a 256`. A change forces a new instance."
  value       = local.yact_binary_sha256
}
