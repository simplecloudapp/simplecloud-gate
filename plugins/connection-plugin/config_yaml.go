package connection

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Each migration upgrades one version to the next. Add steps here when the
// schema changes; migrations run in memory and never overwrite users' files.
type configMigration func(*yaml.Node) error

var configMigrations = map[int]configMigration{}

func decodeConfigYAML(contents []byte, target any) error {
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return err
	}
	if len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("configuration must be a YAML mapping")
	}
	if err := migrateConfig(root, configVersion, configMigrations); err != nil {
		return err
	}
	if err := normalizeConfigKeys(root, reflect.TypeOf(target)); err != nil {
		return err
	}
	return root.Decode(target)
}

func migrateConfig(root *yaml.Node, current int, migrations map[int]configMigration) error {
	var versionNode *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "version" {
			if versionNode != nil {
				return fmt.Errorf("version is configured more than once")
			}
			versionNode = root.Content[i+1]
		}
	}
	// Early Gate configs could omit the version. Their schema is version 2,
	// even after the current schema changes in a future release.
	if versionNode == nil {
		versionNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "2"}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"}, versionNode)
	}
	version, err := strconv.Atoi(versionNode.Value)
	if versionNode.Kind != yaml.ScalarNode || err != nil || version < 1 {
		return fmt.Errorf("config version must be a positive integer")
	}
	if version > current {
		return fmt.Errorf("config version %d is newer than supported version %d; update SimpleCloud Gate", version, current)
	}
	for next := version; next < current; next++ {
		if migrations[next] == nil {
			return fmt.Errorf("no config migration from version %d to %d; use version %d configs from the original connection plugin", next, next+1, current)
		}
	}
	for next := version; next < current; next++ {
		if err := migrations[next](root); err != nil {
			return fmt.Errorf("migrate config from version %d to %d: %w", next, next+1, err)
		}
		versionNode.Value = strconv.Itoa(next + 1)
	}
	// Configurate also accepts quoted version numbers.
	versionNode.Tag = "!!int"
	return nil
}

// Normalize schema fields only. User-defined keys, such as message variables,
// are data and must retain their spelling. Aliases and YAML merges are decoded
// into fresh nodes first so the same anchor can be used in different contexts.
func normalizeConfigKeys(node *yaml.Node, schema reflect.Type) error {
	for schema.Kind() == reflect.Pointer {
		schema = schema.Elem()
	}
	if node.Kind == yaml.AliasNode {
		var value any
		if err := node.Decode(&value); err != nil {
			return err
		}
		if err := node.Encode(value); err != nil {
			return err
		}
	}
	switch schema.Kind() {
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			return nil
		}
		fields := make(map[string]reflect.StructField)
		for i := 0; i < schema.NumField(); i++ {
			field := schema.Field(i)
			key := field.Tag.Get("yaml")
			fields[key] = field
			fields[camelConfigKey(key)] = field
		}
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Tag == "!!merge" {
				if value.Kind == yaml.SequenceNode {
					for _, merged := range value.Content {
						if err := normalizeConfigKeys(merged, schema); err != nil {
							return err
						}
					}
				} else if err := normalizeConfigKeys(value, schema); err != nil {
					return err
				}
				continue
			}
			field, ok := fields[key.Value]
			if !ok {
				continue
			}
			canonical := field.Tag.Get("yaml")
			if seen[canonical] {
				return fmt.Errorf("line %d: %s is configured more than once (including camelCase aliases)", key.Line, canonical)
			}
			seen[canonical] = true
			key.Value = canonical
			if err := normalizeConfigKeys(value, field.Type); err != nil {
				return fmt.Errorf("%s: %w", canonical, err)
			}
		}
	case reflect.Slice:
		if node.Kind == yaml.SequenceNode {
			for _, value := range node.Content {
				if err := normalizeConfigKeys(value, schema.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func camelConfigKey(key string) string {
	parts := strings.Split(key, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func defaultCommandMessages() CommandMessages {
	return CommandMessages{
		AlreadyConnected:        "<color:#dc2626>You are already connected to this server!",
		NoTargetConnectionFound: "<color:#dc2626>Couldn't find a target server!",
	}
}

func (c *CommandEntry) UnmarshalYAML(node *yaml.Node) error {
	type plain CommandEntry
	value := plain{Messages: defaultCommandMessages()}
	if err := node.Decode(&value); err != nil {
		return err
	}
	*c = CommandEntry(value)
	return nil
}

func (c *NetworkJoinTargetsConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain NetworkJoinTargetsConfig
	value := plain{Enabled: true, TargetConnections: []TargetConnection{}}
	if err := node.Decode(&value); err != nil {
		return err
	}
	*c = NetworkJoinTargetsConfig(value)
	return nil
}

func (c *FallbackConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain FallbackConfig
	value := plain{Enabled: true, TargetConnections: []FallbackTargetConnection{}}
	if err := node.Decode(&value); err != nil {
		return err
	}
	*c = FallbackConfig(value)
	return nil
}
