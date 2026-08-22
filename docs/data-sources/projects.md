---
page_title: "revenuecat_projects Data Source - revenuecat"
subcategory: ""
description: |-
  Lists every RevenueCat project accessible with the configured API key.
---

# revenuecat_projects (Data Source)

Lists every RevenueCat project accessible with the configured API key.

## Example Usage

```terraform
data "revenuecat_projects" "all" {}

output "project_names" {
  value = [for project in data.revenuecat_projects.all.projects : project.name]
}
```

## Schema

### Read-Only

- `projects` (List of Object) The accessible projects, each with:
  - `id` (String) Identifier of the project.
  - `name` (String) Name of the project.
  - `created_at` (Number) Creation time of the project, in milliseconds since the Unix epoch.
