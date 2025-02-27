---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "stackit_rabbitmq_credentials Data Source - stackit"
subcategory: ""
description: |-
  RabbitMQ credentials data source schema.
---

# stackit_rabbitmq_credentials (Data Source)

RabbitMQ credentials data source schema.

## Example Usage

```terraform
data "stackit_rabbitmq_credentials" "example" {
  project_id     = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  instance_id    = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  credentials_id = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `credentials_id` (String) The credentials ID.
- `instance_id` (String) ID of the RabbitMQ instance.
- `project_id` (String) STACKIT project ID to which the instance is associated.

### Read-Only

- `host` (String)
- `hosts` (List of String)
- `http_api_uri` (String)
- `id` (String) Terraform's internal resource identifier.
- `name` (String)
- `password` (String, Sensitive)
- `port` (Number)
- `uri` (String)
- `username` (String)
