---
page_title: "revenuecat_entitlement_product_attachment Resource - revenuecat"
subcategory: ""
description: |-
  Attaches products to a RevenueCat entitlement.
---

# revenuecat_entitlement_product_attachment (Resource)

Attaches products to a RevenueCat entitlement, granting the entitlement to customers who purchase
them.

~> **Only one attachment resource may manage a given entitlement.** This resource owns the entire
set of products attached to `entitlement_id`. Declaring a second
`revenuecat_entitlement_product_attachment` for the same entitlement makes the two resources fight
over that set, producing a permanent diff. The provider cannot detect this, because Terraform gives
a resource no view of its siblings.

When the set changes, the provider attaches only the newly configured products and detaches only
the removed ones. Products present before and after are left alone, so adding one product does not
momentarily revoke access for customers holding the others.

Destroying this resource detaches the products; it does not delete the products or the entitlement.

## Example Usage

```terraform
resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = data.revenuecat_project.main.id
  entitlement_id = revenuecat_entitlement.pro.id

  product_ids = [
    revenuecat_product.monthly_ios.id,
    revenuecat_product.monthly_android.id,
  ]
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the entitlement belongs to. Changing this forces a new attachment.
- `entitlement_id` (String) Identifier of the entitlement the products are attached to. Changing this forces a new attachment.
- `product_ids` (Set of String) Identifiers of the products attached to the entitlement. Must not be
  empty; destroy the resource instead of emptying the set.

### Read-Only

- `id` (String) Synthetic identifier of the attachment, in the form `<project_id>:<entitlement_id>`.

## Import

Attachments are imported with a `<project_id>:<entitlement_id>` identifier:

```shell
terraform import revenuecat_entitlement_product_attachment.pro proj1abc:entl1xyz
```
