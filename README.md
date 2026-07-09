# LiteLLM Terraform Provider (release mirror)

This repository is a thin release mirror. The provider's source of truth lives at
[`terraform/provider/` in BerriAI/litellm](https://github.com/BerriAI/litellm/tree/main/terraform/provider),
where every change is built, tested, and statically audited against the LiteLLM proxy's
OpenAPI schema so the provider cannot drift from the API it manages

Do not open pull requests here; they would be overwritten by the next release sync. Send
code changes to [BerriAI/litellm](https://github.com/BerriAI/litellm) and file bugs on its
[issue tracker](https://github.com/BerriAI/litellm/issues)

Releases are published by mirroring the monorepo directory into this repository and pushing
a `vX.Y.Z` tag, which triggers the `Release` workflow (goreleaser builds, GPG-signed
checksums, GitHub release). The public Terraform Registry ingests those releases as
[`BerriAI/litellm`](https://registry.terraform.io/providers/BerriAI/litellm/latest)

## Using the provider

```hcl
terraform {
  required_providers {
    litellm = {
      source  = "BerriAI/litellm"
      version = "~> 0.2"
    }
  }
}

provider "litellm" {
  api_base = var.litellm_api_base
  api_key  = var.litellm_api_key
}
```

Resource and data source documentation is in [`docs/`](docs/) and rendered on the
[registry page](https://registry.terraform.io/providers/BerriAI/litellm/latest/docs)
