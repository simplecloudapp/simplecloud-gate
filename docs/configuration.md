# Configuration

The plugin uses the same three-file configuration layout as the Java plugin:

- `simplecloud-connection/config.yml`
- `simplecloud-connection/messages.yml`
- `simplecloud-connection/commands.yml`

Set `SIMPLECLOUD_GATE_CONFIG_DIR` to use another directory. Missing files are
created with defaults on startup.

## SimpleCloud API

The Go API reads its connection settings from environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `SIMPLECLOUD_NETWORK_ID` | `default` | SimpleCloud network ID |
| `SIMPLECLOUD_NETWORK_SECRET` | none | Network secret |
| `SIMPLECLOUD_CONTROLLER_URL` | `https://controller.simplecloud.app` | Controller REST URL |
| `SIMPLECLOUD_NATS_URL` | `wss://nats.simplecloud.app:443` | Controller event URL |
| `SIMPLECLOUD_COMPONENT` | `simplecloud-gate` | Component header |

The Cloud API is only initialized when `registration.enabled` is `true`.

## Registration

Dynamic servers use `<group>-<numerical_id>` by default; persistent servers use
`<name>`. The available placeholders are `<group>`, `<name>`,
`<numerical_id>`, `<id>`, and every server property. Property keys are lowercased
and characters outside `a-z`, `0-9`, and `_` become `_`.

When registration is enabled, the plugin clears Gate's static server registry,
adds `registration.additional-servers`, loads all currently AVAILABLE game
servers, and then follows SimpleCloud state-change and stop events.

## Connections and matchers

Each connection selects Gate servers through a matcher. Supported operations
match the Java plugin: `REGEX`, `PATTERN`, `EQUALS`, `CONTAINS`, `STARTS_WITH`,
`ENDS_WITH`, and `GREATER_THAN`. Matching is case-insensitive except regex
operations. As in the shared Java matcher, `PATTERN` treats the runtime key as
the regex, while string operands do not satisfy `GREATER_THAN`.

Connection rules can be `PERMISSION` or `ENV`, can be negated, and can define a
`bypass-permission`. Address routes intentionally bypass connection rules, just
like the Velocity listener.

Targets are evaluated by descending priority. If multiple servers match one
connection, the server with the fewest players connected to this Gate instance
is selected.

## Commands and messages

`commands.yml` creates proxy commands such as `/lobby` with aliases. Commands
support multiple prioritized targets and `from` restrictions. `/connection
reload` requires `simplecloud.connection.reload` and reloads all three files.
Like the source plugin, reloading updates connection definitions, rules, join
and fallback targets, and general messages. Restart after changing any setting
in `commands.yml` or changing registration settings. Registered commands retain
their startup targets, permissions, aliases, and command-specific messages.

Messages support variables and the MiniMessage tags used by the default
configuration: colors (named or hex), bold, italic, underlined,
strikethrough, obfuscated, reset, and `<br>`/`<newline>`.

## Reusing an existing proxy configuration

Copy the version 2 `config.yml`, `commands.yml`, and `messages.yml` from your
original connection plugin directory into `simplecloud-connection/`. You can
also point `SIMPLECLOUD_GATE_CONFIG_DIR` at the original directory. Keep Gate's
own root `config.yml` separate from the connection plugin's `config.yml`.

All version 2 field names and nesting match the original plugin, including
hyphenated keys such as `server-name-matcher`, `network-join-targets`, and
`target-connections`. Earlier Gate camelCase keys also load. New files use the
original hyphenated spelling. Loading and reloading do not rewrite existing
files. Specifying both spellings of one field is rejected as ambiguous.

Omitted matcher operations default to `STARTS_WITH`. Version 2 rules default to
`PERMISSION` and `EQUALS`. Custom commands inherit the original default messages
when those fields are omitted.

Version numbers may be integers or quoted integers. Files without a version
are treated as version 2. Unsupported versions are rejected before replacing
working configuration. Ordered migration steps have a dedicated entry point in
`plugins/connection-plugin/config_yaml.go`; no version 1 conversion is included.

### Remaining compatibility limits

- Message rendering supports the subset listed above, including `<red>` and
  `<color:red>`. Full Adventure MiniMessage features such as gradients, hover,
  click events, and translatable components are not implemented.
- `REGEX` and `PATTERN` use Go's regular expressions. Java-only constructs such
  as lookarounds and backreferences are unsupported. Invalid configured `REGEX`
  values fail validation.
- Permission checks use Gate's permission system. Java permission plugins and
  their data are not loaded by this binary.
- Velocity's own listener, forwarding, and authentication configuration must be
  translated into Gate's root config. Only the connection plugin's three files
  share the format.

This comparison uses the original `server-connection-plugin` version 2 config
classes, Velocity listeners and command manager, and the SimpleCloud plugin
documentation. Runtime integration with live Minecraft clients and a controller
still needs deployment testing.
