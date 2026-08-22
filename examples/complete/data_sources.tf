# Reading back parts of the catalog this configuration does not manage.

data "revenuecat_projects" "all" {}

data "revenuecat_entitlement" "pro_by_key" {
  project_id = data.revenuecat_project.main.id
  lookup_key = revenuecat_entitlement.pro.lookup_key

  depends_on = [revenuecat_entitlement.pro]
}

output "accessible_project_names" {
  description = "Names of every project the API key can reach."
  value       = [for project in data.revenuecat_projects.all.projects : project.name]
}

output "pro_entitlement_display_name" {
  description = "Display name resolved by looking the entitlement up by its lookup key."
  value       = data.revenuecat_entitlement.pro_by_key.display_name
}
