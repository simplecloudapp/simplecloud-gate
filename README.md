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

SimpleCloud Gate is [Minekube Gate][gate] with SimpleCloud plugins built in.

## Features

- [x] Automatic SimpleCloud server registration
- [x] Player routing, join targets, and kick fallbacks
- [x] Configurable commands such as `/lobby`
- [x] Support for existing Connection plugin v2 configs

## Installation

Download the binary for your platform from [GitHub Releases][releases].
On Linux or macOS, make it executable and start it with your network settings:

```sh
chmod +x gate_0.74.9_linux_amd64
export SIMPLECLOUD_NETWORK_ID="your-network-id"
export SIMPLECLOUD_NETWORK_SECRET="your-network-secret"
./gate_0.74.9_linux_amd64
```

Use the filename for your platform. Configuration files are created in
`simplecloud-connection/` on first startup. By default, joins, `/lobby`, and
fallbacks use servers whose names start with `lobby`.

See the [configuration guide][docs-thisproject] for settings and compatibility.

## Using your own plugins

In your Gate project, add our Connection plugin as a dependency:

```sh
go get github.com/simplecloudapp/simplecloud-gate@v0.74.9
```

Import it in your entry point:

```go
import connection "github.com/simplecloudapp/simplecloud-gate/plugins/connection-plugin"
```

Register it alongside your own `proxy.Plugin` before starting Gate:

```go
proxy.Plugins = append(proxy.Plugins, connection.Plugin, myPlugin)
gate.Execute()
```

Replace `myPlugin` with your plugin, then build with `go build .`. Plugins are
compiled into the binary. Our configuration stays in `simplecloud-connection/`.

## Releases

[Depot CI][depot] checks for Gate updates daily. Releases use the same versions,
binary names, and platforms as Gate, with SimpleCloud plugins included.
See [release maintenance](docs/releases.md) for workflow setup and manual runs.

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
