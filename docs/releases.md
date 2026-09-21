# Release maintenance

All CI runs on Depot CI through `.depot/workflows/`. There are no GitHub Actions
workflows. GitHub hosts the repository, version tags, and release downloads.

## Depot setup

The repository uses the SimpleCloud Depot organization (`3nccvg6v98`). Depot's
Code Access GitHub App must have access to `simplecloudapp/simplecloud-gate`.
The organization-wide `GH_TOKEN` secret supplies GitHub write access for commits,
tags, and release uploads, following the existing SimpleCloud Depot setup.
Build and test steps use read-only permissions and do not receive this secret.

After workflows reach `main`, Depot registers their push, pull request, daily
schedule, and manual triggers. Check them with:

```sh
depot ci workflow list --org 3nccvg6v98 --repo simplecloudapp/simplecloud-gate
depot ci run list --org 3nccvg6v98 --repo simplecloudapp/simplecloud-gate
```

## Daily Gate update

`update-gate.yml` runs at **04:17 UTC every day**. It reads Gate's latest published
stable release, refuses downgrades, updates `go.mod` and `go.sum`, and copies the
GoReleaser configuration from that exact upstream tag. The project name stays
`gate`, so asset names match upstream.

The workflow runs vet, tests with the race detector, script tests, and a complete
snapshot release build before committing anything. It pushes `main` and the
matching tag atomically. Concurrent updates to `main` cause a failed push rather
than overwriting changes. Existing tags are never moved. The shared `GH_TOKEN`
allows the resulting push to trigger the Depot release workflow.

An upstream change that breaks the plugin API or build tooling fails the update
without pushing it. Such a change needs a compatibility fix before retrying.

To run the daily workflow manually:

```sh
depot ci dispatch --org 3nccvg6v98 --repo simplecloudapp/simplecloud-gate \
  --workflow update-gate.yml --ref main
```

## Binary releases

The release tag must exactly match `go list -m go.minekube.com/gate`, for example
`v0.74.9`. No independent SimpleCloud version counter is used. Plugin changes
land on `main`; a published version is immutable and is not rebuilt in place.

`ci.yml` builds snapshots for branch and PR checks. On a version tag it:

1. Runs the tests and verifies that the tag matches the Gate dependency.
2. Cross-compiles this repository's entry point with upstream's GoReleaser matrix.
3. Checks binary names against the matching upstream release and verifies every checksum.
4. Uploads binaries and `checksums.txt` to a draft GitHub release.
5. Downloads the uploaded assets, checks their hashes, and publishes the draft.

An interrupted draft can be retried. An already published release cannot be
overwritten by this workflow. The binaries include the Connection plugin;
configuration files are created at startup and aren't added to the binary assets.

To prepare a first release after testing:

```sh
version=$(go list -m -f '{{.Version}}' go.minekube.com/gate)
python3 scripts/gate_release.py check-tag "$version"
git tag "$version"
git push origin "$version"
```

The [upstream release configuration](https://github.com/minekube/gate/blob/v0.74.9/.goreleaser.yml)
is the source for the initial platform matrix. Depot's
[CI documentation](https://depot.dev/docs/ci/overview) explains workflow triggers
and sandbox types. Depot CI uses Linux sandboxes; GoReleaser cross-compiles the
Windows and macOS binaries there.
