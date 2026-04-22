# Release Process

This document describes the release process for the LiteLLM Terraform Provider.

## Overview

Releases are triggered manually via GitHub Actions using [semantic-release](https://semantic-release.gitbook.io/semantic-release/). It analyzes commits since the last tag, determines the next version, creates a git tag, and then the existing GoReleaser workflow fires automatically to build, sign, and publish the provider to GitHub Releases and the Terraform Registry.

## How to Release

### 1. Prepare the Release

Before triggering the release:

1. **Update CHANGELOG.md**
   - Move items from `[Unreleased]` section to a new version section
   - Follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format
   - Include all notable changes since the last release

   Example:
   ```markdown
   ## [0.2.1] - 2026-04-08

   ### Fixed
   - Bug fix description

   ### Added
   - New feature description
   ```

2. **Verify tests pass**
   ```bash
   make test
   ```

3. **Verify the build works locally**
   ```bash
   make build
   ```

4. **Commit and push changes to `main`**
   ```bash
   git add CHANGELOG.md
   git commit -m "docs: update CHANGELOG for vX.Y.Z"
   git push upstream main
   ```

### 2. Trigger the Release

1. Go to **Actions → Semantic Release → Run workflow** in the GitHub UI
2. Click **Run workflow**

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

### GPG Key Setup

The Terraform Registry requires all providers to be signed with a GPG key. This must be configured before the first release.

#### 1. Generate a GPG Key

If you don't already have a GPG key for provider signing:

```bash
gpg --full-generate-key
```

Configuration:
- Key type: RSA and RSA (default)
- Key size: 4096 bits
- Expiration: No expiration (or set a long expiration period)
- Email: Use an email associated with your GitHub account
- Set a strong passphrase (or leave empty for CI/CD use)

#### 2. Export the GPG Key

```bash
# List your keys to get the key ID
gpg --list-secret-keys --keyid-format=long

# Example output:
# sec   rsa4096/ABCD1234EFGH5678 2024-01-01 [SC]
#       1234567890ABCDEF1234567890ABCDEF12345678
# uid                 [ultimate] Your Name <your.email@example.com>
#
# The key ID is: ABCD1234EFGH5678
# The fingerprint is: 1234567890ABCDEF1234567890ABCDEF12345678

# Export the private key (ASCII-armored format)
gpg --armor --export-secret-keys ABCD1234EFGH5678

# Export the public key
gpg --armor --export ABCD1234EFGH5678
```

#### 3. Configure GitHub Repository Secrets

Add the following secrets to the repository at: **Settings → Secrets and variables → Actions → New repository secret**

| Secret Name | Description | Value |
|-------------|-------------|-------|
| `GPG_PRIVATE_KEY` | The GPG private key for signing releases | Full output from `gpg --armor --export-secret-keys` (including `-----BEGIN PGP PRIVATE KEY BLOCK-----` and `-----END PGP PRIVATE KEY BLOCK-----`) |
| `PASSPHRASE` | The passphrase for the GPG key | Your GPG key passphrase (leave empty if no passphrase was set) |

#### 4. Register Public Key with Terraform Registry

Before publishing to the Terraform Registry:

1. Go to https://registry.terraform.io/settings/gpg-keys
2. Click "Add a key"
3. Paste your public GPG key (output from `gpg --armor --export`)
4. Submit

**Note**: The public key fingerprint must match the key used to sign the provider releases.

### 3. Monitor the Release Workflow

1. Go to https://github.com/BerriAI/terraform-provider-litellm/actions
2. Find the **Semantic Release** workflow run and confirm the tag was created
3. Find the **Release** (GoReleaser) workflow run triggered by the new tag and monitor its progress

The GoReleaser workflow will:
- Build binaries for multiple platforms (Linux, macOS, Windows, FreeBSD)
- Create archives and checksums
- Sign the checksums with GPG
- Create a GitHub Release and upload all artifacts

### 4. Verify the Release

After both workflows complete successfully:

1. **Check the GitHub Release** at https://github.com/BerriAI/terraform-provider-litellm/releases — confirm binaries, SHA256SUMS, and `.sig` are present

2. **Verify the signature** (optional)
   ```bash
   wget https://github.com/BerriAI/terraform-provider-litellm/releases/download/v0.1.2/terraform-provider-litellm_0.1.2_SHA256SUMS
   wget https://github.com/BerriAI/terraform-provider-litellm/releases/download/v0.1.2/terraform-provider-litellm_0.1.2_SHA256SUMS.sig

   gpg --verify terraform-provider-litellm_0.1.2_SHA256SUMS.sig terraform-provider-litellm_0.1.2_SHA256SUMS
   ```

3. **Check the Terraform Registry** at https://registry.terraform.io/providers/BerriAI/litellm/latest — the new version should appear within a few minutes

## Troubleshooting

### semantic-release Creates No Tag

**Cause**: No commits since the last tag match `fix:`, `feat:`, or `BREAKING CHANGE`.

**Solution**: Check commit messages since the last tag. At least one must follow the Conventional Commits format with a release-triggering prefix.

### Release Workflow Fails with GPG Error

**Error**: `Input required and not supplied: gpg_private_key`

**Solution**:
- Verify that `GPG_PRIVATE_KEY` and `PASSPHRASE` secrets are configured in the repository
- Ensure the secrets are not expired
- Check that the secret names match exactly (case-sensitive)

### GoReleaser Signing Fails

**Error**: `gpg: signing failed: No secret key`

**Solution**:
- Verify the `GPG_PRIVATE_KEY` secret contains the complete private key block
- Ensure the passphrase is correct
- Check that the key hasn't expired: `gpg --list-keys`

### Build Fails

**Error**: Build errors during compilation

**Solution**:
- Run `make test` and `make build` locally first
- Ensure `go.mod` and `go.sum` are up to date
- Check that all dependencies are available

### Tag Already Exists

**Error**: Cannot push tag because it already exists

**Solution**:
```bash
# Delete local tag
git tag -d v0.1.2

# Delete remote tag (use with caution!)
git push origin :refs/tags/v0.1.2
```

**Warning**: Deleting and recreating tags should be avoided in production. If a release has already been published, create a new patch version instead.

### Registry Not Showing New Version

The registry webhook usually picks up within minutes. If not, check the webhook status under the provider's settings on registry.terraform.io.

## Version Numbering

This project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html):

- **MAJOR** version (1.0.0): Incompatible API changes
- **MINOR** version (0.1.0): New functionality in a backward-compatible manner
- **PATCH** version (0.0.1): Backward-compatible bug fixes

For pre-1.0 releases:
- Breaking changes may occur in minor versions
- Patch versions should only contain bug fixes

## Security Considerations

1. **Never commit private keys**: The GPG private key should only be stored as a GitHub secret
2. **Protect repository secrets**: Limit who has access to manage repository secrets
3. **Use a dedicated key**: Consider using a separate GPG key specifically for provider signing
4. **Key rotation**: If the GPG key is compromised, generate a new key, update secrets, and register the new public key with the Terraform Registry
5. **Passphrase**: Use a strong passphrase for the GPG key, or use a passphrase-less key specifically for CI/CD

## References

- [semantic-release Documentation](https://semantic-release.gitbook.io/semantic-release/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [GoReleaser Documentation](https://goreleaser.com/)
- [Terraform Provider Publishing](https://www.terraform.io/docs/registry/providers/publishing.html)
- [HashiCorp GPG Signing Requirements](https://www.terraform.io/docs/registry/providers/publishing.html#signing-releases)
- [GitHub Actions Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- [Semantic Versioning](https://semver.org/)
