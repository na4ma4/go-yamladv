package yamladv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/na4ma4/go-yamladv"
	"go.yaml.in/yaml/v3"
)

func TestInclude(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Write included file
	os.WriteFile(filepath.Join(tmpDir, "roles.yaml"), []byte(`
- name: "ro"
  server:
    '*': [ "usage" ]
`), 0o644)

	// Write main file
	mainYAML := `roles: !include roles.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type roleConfig struct {
		Name   string              `yaml:"name"`
		Server map[string][]string `yaml:"server"`
	}
	type config struct {
		Roles []roleConfig `yaml:"roles"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Roles) != 1 || cfg.Roles[0].Name != "ro" {
		t.Errorf("expected 1 role 'ro', got %v", cfg.Roles)
	}
}

func TestIncludeNested(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Write nested include
	os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, "sub", "inner.yaml"), []byte(`key: "inner_value"`), 0o644)

	// Write middle include
	os.WriteFile(filepath.Join(tmpDir, "middle.yaml"), []byte(`nested: !include sub/inner.yaml`), 0o644)

	mainYAML := `top: !include middle.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type inner struct {
		Key string `yaml:"key"`
	}
	type middle struct {
		Nested inner `yaml:"nested"`
	}
	type top struct {
		Top middle `yaml:"top"`
	}
	var cfg top
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Top.Nested.Key != "inner_value" {
		t.Errorf("expected inner_value, got %q", cfg.Top.Nested.Key)
	}
}

func TestIncludeCircular(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// a.yaml includes b.yaml, b.yaml includes a.yaml
	os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte(`next: !include b.yaml`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte(`next: !include a.yaml`), 0o644)

	mainYAML := `start: !include a.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	err := yamladv.Resolve(&root, tmpDir)
	if err == nil {
		t.Fatal("expected error for circular include")
	}
	if !contains(err.Error(), "circular") {
		t.Errorf("expected circular error, got: %v", err)
	}
}

func TestIncludeDirList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "items")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(`name: "second"`), 0o644)
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(`name: "first"`), 0o644)

	mainYAML := `items: !include_dir_list items/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type item struct {
		Name string `yaml:"name"`
	}
	type config struct {
		Items []item `yaml:"items"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cfg.Items))
	}
	// Sorted alphanumerically: a.yaml before b.yaml
	if cfg.Items[0].Name != "first" {
		t.Errorf("expected first item 'first', got %q", cfg.Items[0].Name)
	}
	if cfg.Items[1].Name != "second" {
		t.Errorf("expected second item 'second', got %q", cfg.Items[1].Name)
	}
}

func TestIncludeDirNamed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "servers")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "hyperion.yaml"), []byte(`
host: "hyperion.example.com"
port: 3306
`), 0o644)
	os.WriteFile(filepath.Join(dir, "titan.yaml"), []byte(`
host: "titan.example.com"
enabled: false
`), 0o644)

	mainYAML := `servers: !include_dir_named servers/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type server struct {
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
		Enabled bool   `yaml:"enabled"`
	}
	type config struct {
		Servers map[string]server `yaml:"servers"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}
	if cfg.Servers["hyperion"].Host != "hyperion.example.com" {
		t.Errorf("expected hyperion host, got %v", cfg.Servers["hyperion"])
	}
	if cfg.Servers["titan"].Host != "titan.example.com" {
		t.Errorf("expected titan host, got %v", cfg.Servers["titan"])
	}
}

func TestIncludeDirMergeList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "automation")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "lights.yaml"), []byte(`
- alias: "light_on"
- alias: "light_off"
`), 0o644)
	os.WriteFile(filepath.Join(dir, "sensors.yaml"), []byte(`
- alias: "sensor_read"
`), 0o644)

	mainYAML := `automation: !include_dir_merge_list automation/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type auto struct {
		Alias string `yaml:"alias"`
	}
	type config struct {
		Automation []auto `yaml:"automation"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Automation) != 3 {
		t.Fatalf("expected 3 merged list entries, got %d: %v", len(cfg.Automation), cfg.Automation)
	}
}

func TestIncludeDirMergeListNotList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "bad")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "notlist.yaml"), []byte(`key: "value"`), 0o644)

	mainYAML := `items: !include_dir_merge_list bad/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	err := yamladv.Resolve(&root, tmpDir)
	if err == nil {
		t.Fatal("expected error for non-list file in merge_list")
	}
	if !contains(err.Error(), "must contain a list") {
		t.Errorf("expected 'must contain a list' error, got: %v", err)
	}
}

func TestIncludeDirMergeNamed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "group")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "interior.yaml"), []byte(`
bedroom:
  name: "Bedroom"
hallway:
  name: "Hallway"
`), 0o644)
	os.WriteFile(filepath.Join(dir, "exterior.yaml"), []byte(`
front_yard:
  name: "Front Yard"
`), 0o644)

	mainYAML := `group: !include_dir_merge_named group/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type groupEntry struct {
		Name string `yaml:"name"`
	}
	type config struct {
		Group map[string]groupEntry `yaml:"group"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Group) != 3 {
		t.Fatalf("expected 3 merged map entries, got %d: %v", len(cfg.Group), cfg.Group)
	}
	if cfg.Group["bedroom"].Name != "Bedroom" {
		t.Errorf("expected bedroom, got %v", cfg.Group["bedroom"])
	}
	if cfg.Group["front_yard"].Name != "Front Yard" {
		t.Errorf("expected front_yard, got %v", cfg.Group["front_yard"])
	}
}

func TestIncludeDirMergeNamedNotMapping(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "bad")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "notmap.yaml"), []byte(`- item1`), 0o644)

	mainYAML := `items: !include_dir_merge_named bad/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	err := yamladv.Resolve(&root, tmpDir)
	if err == nil {
		t.Fatal("expected error for non-mapping file in merge_named")
	}
	if !contains(err.Error(), "must contain a mapping") {
		t.Errorf("expected 'must contain a mapping' error, got: %v", err)
	}
}

func TestIncludeDirRecursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "automation")
	os.MkdirAll(filepath.Join(dir, "lights"), 0o755)
	os.MkdirAll(filepath.Join(dir, "sensors"), 0o755)

	os.WriteFile(filepath.Join(dir, "say_hello.yaml"), []byte(`name: "hello"`), 0o644)
	os.WriteFile(filepath.Join(dir, "lights", "on.yaml"), []byte(`name: "light_on"`), 0o644)
	os.WriteFile(filepath.Join(dir, "lights", "off.yaml"), []byte(`name: "light_off"`), 0o644)
	os.WriteFile(filepath.Join(dir, "sensors", "read.yaml"), []byte(`name: "sensor_read"`), 0o644)

	mainYAML := `items: !include_dir_list automation/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type item struct {
		Name string `yaml:"name"`
	}
	type config struct {
		Items []item `yaml:"items"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Items) != 4 {
		t.Fatalf("expected 4 items (recursive), got %d: %v", len(cfg.Items), cfg.Items)
	}
}

func TestYMLExtension(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "items")
	os.MkdirAll(dir, 0o755)

	os.WriteFile(filepath.Join(dir, "a.yml"), []byte(`name: "yml_entry"`), 0o644)

	mainYAML := `items: !include_dir_list items/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type item struct {
		Name string `yaml:"name"`
	}
	type config struct {
		Items []item `yaml:"items"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Items) != 1 || cfg.Items[0].Name != "yml_entry" {
		t.Errorf("expected 1 yml item, got %v", cfg.Items)
	}
}

func TestIncludeMissingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	mainYAML := `data: !include nonexistent.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	err := yamladv.Resolve(&root, tmpDir)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestIncludeDirMergeNamedWithEmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "servers")
	os.MkdirAll(dir, 0o755)

	// !include_dir_merge_named merges file contents by top-level key,
	// so the file must contain a mapping with server names as keys
	os.WriteFile(filepath.Join(dir, "hyperion.yaml"), []byte(`
hyperion:
  host: "hyperion.example.com"
`), 0o644)
	os.WriteFile(filepath.Join(dir, "empty.yaml"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "comments.yaml"), []byte("# just a comment\n"), 0o644)

	mainYAML := `servers: !include_dir_merge_named servers/`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	type server struct {
		Host string `yaml:"host"`
	}
	type config struct {
		Servers map[string]server `yaml:"servers"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server (empty files skipped), got %d: %v", len(cfg.Servers), cfg.Servers)
	}
	if cfg.Servers["hyperion"].Host != "hyperion.example.com" {
		t.Errorf("expected hyperion, got %v", cfg.Servers)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func TestDiamondIncludeNotCircular(t *testing.T) {
	t.Parallel()

	// common.yaml is included from two separate branches of the tree.
	// This is a diamond dependency, NOT circular, and must succeed.
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "common.yaml"), []byte(`value: "shared"`), 0o644)

	mainYAML := `
left: !include common.yaml
right: !include common.yaml
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve should accept diamond includes, got: %v", err)
	}

	type entry struct {
		Value string `yaml:"value"`
	}
	type config struct {
		Left  entry `yaml:"left"`
		Right entry `yaml:"right"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Left.Value != "shared" || cfg.Right.Value != "shared" {
		t.Errorf("expected both sides 'shared', got left=%q right=%q", cfg.Left.Value, cfg.Right.Value)
	}
}

func TestNestedDiamondInclude(t *testing.T) {
	t.Parallel()

	// a.yaml includes common.yaml, b.yaml includes common.yaml.
	// main includes both a.yaml and b.yaml. Not circular.
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "common.yaml"), []byte(`val: "shared"`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte(`sub: !include common.yaml`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte(`sub: !include common.yaml`), 0o644)

	mainYAML := `
first: !include a.yaml
second: !include b.yaml
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve should accept nested diamond includes, got: %v", err)
	}

	type inner struct {
		Val string `yaml:"val"`
	}
	type wrapper struct {
		Sub inner `yaml:"sub"`
	}
	type config struct {
		First  wrapper `yaml:"first"`
		Second wrapper `yaml:"second"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.First.Sub.Val != "shared" || cfg.Second.Sub.Val != "shared" {
		t.Errorf("expected both sides 'shared', got first=%q second=%q", cfg.First.Sub.Val, cfg.Second.Sub.Val)
	}
}

func TestCircularIncludeStillDetected(t *testing.T) {
	t.Parallel()

	// Verify the fix doesn't break actual circular detection.
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte(`next: !include b.yaml`), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte(`next: !include a.yaml`), 0o644)

	mainYAML := `start: !include a.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	err := yamladv.Resolve(&root, tmpDir)
	if err == nil {
		t.Fatal("expected error for circular include")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular error, got: %v", err)
	}
}

func TestIncludedNodePreservesAnchor(t *testing.T) {
	t.Parallel()

	// An included file defines a YAML anchor on its root node.
	// replaceNode must preserve the Anchor field so aliases work.
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "defaults.yaml"), []byte(`&defaults
key: "value"
`), 0o644)

	mainYAML := `defaults: !include defaults.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Walk the resolved tree to find the node that replaced the !include tag
	// and verify its Anchor field was preserved.
	found := false
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Anchor == "defaults" {
			found = true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)

	if !found {
		t.Error("expected to find anchor 'defaults' in resolved tree, but it was missing")
	}
}

func TestIncludedNodePreservesComments(t *testing.T) {
	t.Parallel()

	// An included file has a head comment on the key. replaceNode must preserve it.
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "commented.yaml"), []byte(`# this is a head comment
key: "value"
`), 0o644)

	mainYAML := `section: !include commented.yaml`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(mainYAML), &root); err != nil {
		t.Fatalf("parse main: %v", err)
	}

	if err := yamladv.Resolve(&root, tmpDir); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// yaml.v3 attaches head comments to key nodes inside mappings.
	// Walk the resolved tree and verify the comment survived replaceNode.
	found := false
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && n.Value == "key" && n.HeadComment == "# this is a head comment" {
			found = true
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)

	if !found {
		t.Error("expected to find HeadComment on included key node, but it was missing")
	}
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
