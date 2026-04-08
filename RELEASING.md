# Release Process

This document describes the release process for the LiteLLM Terraform Provider.

## Overview

Releases are triggered manually via GitHub Actions using [semantic-release](https://semantic-release.gitbook.io/semantic-release/). It analyzes commits since the last tag, determines the next version, creates a git tag, and then the existing GoReleaser workflow fires automatically to build, sign, and publish the provider to GitHub Releases and the Terraform Registry.

## How to Release

1. Ensure all PRs for the release are merged to `main`
2. Go to **Actions → Semantic Release → Run workflow** in the GitHub UI
3. Click **Run workflow**

semantic-release will:
- Analyze commits since the last tag using [Conventional Commits](https://www.conventionalcommits.org/)
- Determine the next version (`fix:` → patch, `feat:` → minor, `feat!:` or `BREAKING CHANGE:` → major)
- Create and push the git tag
- Trigger the GoReleaser workflow, which builds binaries for all platforms, signs them with GPG, and publishes the GitHub Release
- The Terraform Registry picks up the new release automatically

## Commit Message Convention

semantic-release determines the version bump from commit messages:

| Commit prefix | Release type | Example |
|---|---|---|
| `fix:` | Patch (0.0.x) | `fix: handle nil pointer in team read` |
| `feat:` | Minor (0.x.0) | `feat: add vector store resource` |
| `feat!:` or `BREAKING CHANGE:` in body | Major (x.0.0) | `feat!: remove deprecated key attribute` |

Commits with other prefixes (`docs:`, `chore:`, `refactor:`, etc.) do not trigger a release.

## Prerequisites (One-Time Setup)

### GPG Key

The Terraform Registry requires signed releases. The following secrets must be configured in the repository under **Settings → Secrets and variables → Actions**:

| Secret | Description |
|---|---|
| `GPG_PRIVATE_KEY` | Full output of `gpg --armor --export-secret-keys <KEY_ID>` |
| `PASSPHRASE` | Passphrase for the GPG key (leave empty if none) |

### Terraform Registry

The public GPG key must be registered at https://registry.terraform.io/settings/gpg-keys. This is a one-time step per signing key.

## Verifying a Release

After the workflow completes:

1. Check the GitHub Release at https://github.com/BerriAI/terraform-provider-litellm/releases — confirm binaries, SHA256SUMS, and `.sig` are present
2. Check the Terraform Registry at https://registry.terraform.io/providers/BerriAI/litellm/latest — the new version should appear within a few minutes

## Troubleshooting

**semantic-release creates no tag**: No commits since the last tag match `fix:`, `feat:`, or `BREAKING CHANGE`. Check commit messages.

**GoReleaser GPG error** (`gpg: signing failed: No secret key`): Verify `GPG_PRIVATE_KEY` and `PASSPHRASE` secrets are set correctly and the key hasn't expired.

**Registry not showing new version**: The registry webhook usually picks up within minutes. If not, check the webhook status under the provider's settings on registry.terraform.io.
