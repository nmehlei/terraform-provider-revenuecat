---
page_title: "revenuecat_package_product_attachment Resource - revenuecat"
subcategory: ""
description: |-
  Attaches products to a RevenueCat package.
---

# revenuecat_package_product_attachment (Resource)

Attaches products to a RevenueCat package, each under an eligibility criteria.

~> **Only one attachment resource may manage a given package.** This resource owns the entire set
of products attached to `package_id`. Declaring a second `revenuecat_package_product_attachment`
for the same package makes the two resources fight over that set, producing a permanent diff.

When the set changes, the provider attaches only newly configured products and detaches only
removed ones. Changing a product's `eligibility_criteria` re-attaches that product rather than
detaching and re-adding it.

Destroying this resource detaches the products; it does not delete the products or the package.

## Example Usage

```terraform
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
```

## Schema

### Required

- `project_id` (String) Identifier of the project the package belongs to. Changing this forces a new attachment.
- `package_id` (String) Identifier of the package the products are attached to. Changing this forces a new attachment.

### Blocks

- `product` (Block Set, Min: 1) A product attached to the package.
  - `product_id` (String, Required) Identifier of the product to attach.
  - `eligibility_criteria` (String, Optional) Eligibility criteria the product is attached under,
    for example `all`.

### Read-Only

- `id` (String) Synthetic identifier of the attachment, in the form `<project_id>:<package_id>`.

## Import

Attachments are imported with a `<project_id>:<package_id>` identifier:

```shell
terraform import revenuecat_package_product_attachment.monthly proj1abc:pkg1def
```
