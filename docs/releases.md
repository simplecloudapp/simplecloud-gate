# Release maintenance

Workflows run on Depot in the SimpleCloud organization (`3nccvg6v98`).
Depot's Code Access GitHub App needs repository access. The shared `GH_TOKEN`
secret supplies write access for commits, tags, and release uploads.

## Daily Gate update

`.depot/workflows/update-gate.yml` runs at **04:17 UTC daily**. It updates the Gate
dependency and GoReleaser configuration to the latest stable release, then runs
tests and builds every binary before pushing `main` and the matching version tag.
Incompatible upstream changes fail the workflow without pushing an update.

To run it manually:

```sh
depot ci dispatch --org 3nccvg6v98 --repo simplecloudapp/simplecloud-gate \
  --workflow update-gate.yml --ref main
```

## Binary releases

`.depot/workflows/ci.yml` tests branch and pull request changes and builds
snapshots. Version tags publish binaries with the same names and platforms as
Gate. The tag must match the Gate dependency, for example `v0.74.9`.

Before publishing, the workflow downloads the uploaded assets and verifies their
checksums. Failed draft releases can be retried; published releases cannot be
overwritten. Changes on `main` do not create a release until a new tag is pushed.

To retry a release, pass the full tag ref:

```sh
depot ci dispatch --org 3nccvg6v98 --repo simplecloudapp/simplecloud-gate \
  --workflow ci.yml --ref refs/tags/v0.74.9
```
