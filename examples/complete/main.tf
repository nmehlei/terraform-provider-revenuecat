# An end-to-end RevenueCat catalog: two store apps, their products, an
# entitlement granted by those products, and an offering whose packages present
# them on a paywall.

terraform {
  required_providers {
    revenuecat = {
      source  = "nmehlei/revenuecat"
      version = "~> 0.1"
    }
  }
}

provider "revenuecat" {
  # Reads REVENUECAT_API_KEY from the environment.
}

variable "project_name" {
  description = "Name of the existing RevenueCat project to configure."
  type        = string
  default     = "Acme"
}

# Projects are created in the RevenueCat dashboard, so this configuration adopts
# an existing one rather than managing it.
data "revenuecat_project" "main" {
  name = var.project_name
}

resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
}

resource "revenuecat_app" "android" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme Android"
  type       = "play_store"
}

resource "revenuecat_product" "monthly_ios" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (iOS)"
}

resource "revenuecat_product" "monthly_android" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.android.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (Android)"
}

resource "revenuecat_product" "annual_ios" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.annual"
  type             = "subscription"
  display_name     = "Pro Annual (iOS)"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}

# One attachment resource owns every product granting the entitlement. Splitting
# these across two resources would make them fight over the same set.
resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = data.revenuecat_project.main.id
  entitlement_id = revenuecat_entitlement.pro.id

  product_ids = [
    revenuecat_product.monthly_ios.id,
    revenuecat_product.monthly_android.id,
    revenuecat_product.annual_ios.id,
  ]
}

resource "revenuecat_offering" "default" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "default"
  display_name = "Default Offering"
  is_current   = true

  metadata = {
    paywall_variant = "a"
  }
}

resource "revenuecat_package" "monthly" {
  project_id   = data.revenuecat_project.main.id
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = "Monthly"
  position     = 1
}

resource "revenuecat_package" "annual" {
  project_id   = data.revenuecat_project.main.id
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_annual"
  display_name = "Annual"
  position     = 2
}

# A package groups the equivalent product on each store, so the paywall offers
# one "Monthly" choice regardless of platform.
resource "revenuecat_package_product_attachment" "monthly" {
  project_id = data.revenuecat_project.main.id
  package_id = revenuecat_package.monthly.id

  product {
    product_id           = revenuecat_product.monthly_ios.id
    eligibility_criteria = "all"
  }

  product {
    product_id           = revenuecat_product.monthly_android.id
    eligibility_criteria = "all"
  }
}

resource "revenuecat_package_product_attachment" "annual" {
  project_id = data.revenuecat_project.main.id
  package_id = revenuecat_package.annual.id

  product {
    product_id           = revenuecat_product.annual_ios.id
    eligibility_criteria = "all"
  }
}

output "entitlement_id" {
  description = "Identifier of the managed entitlement."
  value       = revenuecat_entitlement.pro.id
}

output "current_offering_id" {
  description = "Identifier of the offering marked current."
  value       = revenuecat_offering.default.id
}
