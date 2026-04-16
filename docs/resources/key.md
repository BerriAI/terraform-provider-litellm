# litellm_key Resource

Manages a LiteLLM API key.

> **Requires Terraform 1.11+** — this resource uses write-only attributes to prevent the API key from being stored in Terraform state.

## Example Usage

```hcl
resource "litellm_key" "example" {
  models               = ["gpt-3.5-turbo", "gpt-4"]
  max_budget           = 100.0
  user_id              = "user123"
  team_id              = "team456"
  max_parallel_requests = 5
  metadata             = {
    "environment" = "production"
  }
  tpm_limit            = 1000
  rpm_limit            = 60
  budget_duration      = "monthly"
  allowed_cache_controls = ["no-cache", "max-age=3600"]
  soft_budget          = 80.0
  key_alias            = "prod-key-1"
  duration             = "30d"
  aliases              = {
    "gpt-3.5-turbo" = "chatgpt"
  }
  config               = {
    "default_model" = "gpt-3.5-turbo"
  }
  permissions          = {
    "can_create_keys" = "true"
  }
  model_max_budget     = {
    "gpt-4" = 50.0
  }
  model_rpm_limit      = {
    "gpt-3.5-turbo" = 30
  }
  model_tpm_limit      = {
    "gpt-4" = 500
  }
  guardrails           = ["content_filter", "token_limit"]
  blocked              = false
  tags                 = ["production", "api"]
}

# Capture the key during apply — it won't be available afterward
resource "aws_ssm_parameter" "litellm_key" {
  name  = "/myapp/litellm-key"
  type  = "SecureString"
  value = litellm_key.example.key
}
```

## Argument Reference

The following arguments are supported:

* `models` - (Optional) List of models that can be used with this key. This restricts the key to only use the specified models.

* `max_budget` - (Optional) Maximum budget for this key. This sets an upper limit on the total spend allowed for this key.

* `user_id` - (Optional) User ID associated with this key. This links the key to a specific user in the LiteLLM system.

* `team_id` - (Optional) Team ID associated with this key. This links the key to a specific team in the LiteLLM system.

* `max_parallel_requests` - (Optional) Maximum number of parallel requests allowed for this key. This helps in controlling concurrent usage.

* `metadata` - (Optional) Metadata associated with this key. This can be used to store additional, custom information about the key.

* `tpm_limit` - (Optional) Tokens per minute limit for this key. This sets a rate limit based on the number of tokens processed.

* `rpm_limit` - (Optional) Requests per minute limit for this key. This sets a rate limit based on the number of API calls.

* `budget_duration` - (Optional) Duration for the budget (e.g., "monthly", "weekly"). This defines the time period for which the `max_budget` applies.

* `allowed_cache_controls` - (Optional) List of allowed cache control directives. This can be used to control caching behavior for requests made with this key.

* `soft_budget` - (Optional) Soft budget limit for this key. This can be used to set a warning threshold before reaching the `max_budget`.

* `key_alias` - (Optional) Alias for this key. This provides a human-readable identifier for the key.

* `duration` - (Optional) Duration for which this key is valid. This sets an expiration time for the key.

* `aliases` - (Optional) Map of model aliases. This allows you to create custom names for models when using this key.

* `config` - (Optional) Configuration options for this key. This can be used to set key-specific settings.

* `permissions` - (Optional) Permissions associated with this key. This defines what actions are allowed with this key.

* `model_max_budget` - (Optional) Maximum budget per model. This allows setting different budget limits for each model.

* `model_rpm_limit` - (Optional) Requests per minute limit per model. This allows setting different RPM limits for each model.

* `model_tpm_limit` - (Optional) Tokens per minute limit per model. This allows setting different TPM limits for each model.

* `guardrails` - (Optional) List of guardrails applied to this key. This can be used to enforce certain safety or quality checks.

* `blocked` - (Optional) Whether this key is blocked. If set to true, the key will be unable to make any requests.

* `tags` - (Optional) List of tags associated with this key. This can be used for organization and filtering of keys.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `key` - (Write-only) The generated API key. Available during `terraform apply` but **never stored in Terraform state**. Use it immediately to write to a secrets manager. Cannot be retrieved after the apply completes.

* `token_id` - The SHA-256 hash of the key. Used as the Terraform resource ID. Safe to store in state — cannot be used to authenticate against the LiteLLM API.

* `spend` - The current spend for this key. This reflects the total amount spent using this key so far.

## Security Notes

The `key` attribute is write-only (requires Terraform 1.11+). This means:

- The raw key value is **never written to the Terraform state file**
- It is available during `terraform apply` so you can reference it in other resources (e.g., `aws_ssm_parameter`, `kubernetes_secret`)
- After the apply completes, it cannot be retrieved — plan accordingly

## Import

LiteLLM keys can be imported using the `token_id` (SHA-256 hash of the key), e.g.:

```
$ terraform import litellm_key.example <token_id>
```

The `token_id` can be found via the LiteLLM UI or by calling `GET /key/info?key=<your-key>`.
