# SimpleCloud Gate

![Banner][banner]

<div align="center">

[![Release][badge-release]][releases]
[![License][badge-license]][license]
<br>

[![Discord][badge-discord]][social-discord]
[![Follow @simplecloudapp][badge-x]][social-x]
[![Follow @simplecloudapp][badge-bluesky]][social-bluesky]
[![Follow @simplecloudapp][badge-youtube]][social-youtube]
<br>

[Report a Bug][issue-bug-report]
·
[Request a Feature][issue-feature-request]

</div>
<br>

> Read the [configuration guide][docs-thisproject] for setup and compatibility with existing proxy configs.

SimpleCloud Gate is [Minekube Gate][gate] with SimpleCloud plugins built in.
It currently includes the Connection plugin for server registration, player
routing, and fallback connections.

## Features

- [x] **Server registration**: Discover available SimpleCloud game servers and follow server lifecycle events.
- [x] **Existing configs**: Load the original Connection plugin's version 2 config files, including hyphenated field names.
- [x] **Connection routing**: Select servers through matchers, permissions, environment rules, and virtual-host routes.
- [x] **Join and fallback targets**: Route players by priority and optional source restrictions.
- [x] **Navigation commands**: Configure commands such as `/lobby`, their aliases, permissions, and targets.
- [x] **Messages**: Customize messages with variables, colors, text styles, and line breaks.
- [x] **Gate updates**: Check stable Gate releases daily through Depot CI, test updates, and publish matching binary versions.

## Installation

Download the binary for your platform from [GitHub Releases][releases]. Files use
Gate's naming scheme: `gate_<version>_<os>_<arch>`, with `.exe` on Windows and
`_musl` for Linux musl builds. Each release includes `checksums.txt`.

On Linux or macOS, make the downloaded binary executable and start it with your
SimpleCloud network settings:

```sh
chmod +x gate_0.74.9_linux_amd64
export SIMPLECLOUD_NETWORK_ID="your-network-id"
export SIMPLECLOUD_NETWORK_SECRET="your-network-secret"
./gate_0.74.9_linux_amd64
```

Use the filename for your platform. The plugin creates its three configuration
files in `simplecloud-connection/` on first startup. Copy existing version 2
`config.yml`, `commands.yml`, and `messages.yml` there to reuse them. Gate's own
proxy settings belong in the separate root `config.yml`.

The default connection routes joins, `/lobby`, and kick fallbacks to registered
servers whose names begin with `lobby`. See the [configuration guide][docs-thisproject]
for API settings, custom routes, reload behavior, and remaining MiniMessage and
Java-regex compatibility limits.

## Releases

Release tags follow the bundled Gate version, for example `v0.74.9`. The binaries
use the same names and platform matrix as that upstream release, with our plugins
registered by this repository's entry point.

[Depot CI][depot] runs tests and cross-compiles the binaries. A daily workflow at
04:17 UTC checks for a newer stable Gate release, updates the dependency and
upstream GoReleaser configuration, validates the build, then pushes the update
and matching tag. Releases are published only after uploaded checksums are
verified. See [release maintenance](docs/releases.md) for manual runs and setup.

## Contributing

Read our [Contribution Guide][docs-contribute] before contributing.

```sh
git clone https://github.com/simplecloudapp/simplecloud-gate.git
cd simplecloud-gate
go build ./...
go test ./...
go vet ./...
python3 -m unittest discover -s scripts -p '*_test.py'
```

Use the Go version declared in `go.mod` or newer. Each SimpleCloud plugin has its
own folder under `plugins/`, starting with `plugins/connection-plugin`.

## License

This repository is licensed under the [MIT License][license].

<!-- LINK GROUP -->

[banner]: https://raw.githubusercontent.com/simplecloudapp/branding/main/readme/banner/simplecloudapp.png
[gate]: https://github.com/minekube/gate
[depot]: https://depot.dev/docs/ci/overview
[releases]: https://github.com/simplecloudapp/simplecloud-gate/releases
[issue-bug-report]: https://github.com/simplecloudapp/simplecloud-gate/issues/new?labels=bug
[issue-feature-request]: https://github.com/simplecloudapp/simplecloud-gate/issues/new?labels=enhancement
[docs-thisproject]: docs/configuration.md
[docs-contribute]: https://docs.simplecloud.app/contribute
[license]: ./LICENSE

[social-x]: https://x.com/simplecloudapp
[social-bluesky]: https://bsky.app/profile/simplecloud.app
[social-youtube]: https://www.youtube.com/@thesimplecloud9075
[social-discord]: https://discord.simplecloud.app

[badge-release]: https://img.shields.io/github/v/release/simplecloudapp/simplecloud-gate?style=flat-square&color=0ea5e9
[badge-license]: https://img.shields.io/badge/MIT-blue.svg?style=flat-square&label=license&labelColor=18181b&color=e11d48
[badge-discord]: https://img.shields.io/badge/Community_Discord-d95652.svg?style=flat-square&logo=discord&color=27272a
[badge-x]: https://img.shields.io/badge/Follow_@simplecloudapp-d95652.svg?style=flat-square&logo=x&color=27272a
[badge-bluesky]: https://img.shields.io/badge/Follow_@simplecloud.app-d95652.svg?style=flat-square&logo=bluesky&color=27272a
[badge-youtube]: https://img.shields.io/badge/youtube-d95652.svg?style=flat-square&logo=youtube&color=27272a
