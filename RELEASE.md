# Releasing a new version

## Prerequisites

- Push access to the repository
- Permission to create GitHub releases

## Steps

1. **Ensure `main` is clean and CI passes.**

   All changes for the release should already be merged. Verify the latest
   lint workflow is green.

2. **Choose a version number.**

   Follow [Semantic Versioning](https://semver.org/). For example `v0.2.0`.

3. **Create and push a tag.**

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

4. **Create a GitHub release.**

   Go to **Releases > Draft a new release** (or use the CLI):

   ```bash
   gh release create v0.2.0 --generate-notes
   ```

   The `--generate-notes` flag auto-generates a changelog from merged PRs.

5. **Verify the release workflow.**

   Publishing the release triggers the `release.yml` GitHub Action which
   builds the Docker image and pushes it to GHCR at:

   ```
   ghcr.io/sagadata-public/sagadata-cloud-controller-manager:<version>
   ```

   Check the **Actions** tab to confirm the workflow completes successfully.

## Pulling the image

```bash
docker pull ghcr.io/sagadata-public/sagadata-cloud-controller-manager:0.2.0
```
