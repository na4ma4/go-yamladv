package yamladv_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/na4ma4/go-yamladv"
	"go.yaml.in/yaml/v3"
)

func ExampleResolve() {
	// Set up a temporary config directory with an included file.
	dir, _ := os.MkdirTemp("", "yamladv-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "sensors.yaml"), []byte(`
- name: "temperature"
  unit: "°C"
- name: "humidity"
  unit: "%"
`), 0o644)

	mainYAML := []byte(`sensors: !include sensors.yaml`)

	// Parse into a yaml.Node tree, then resolve includes.
	var root yaml.Node
	if err := yaml.Unmarshal(mainYAML, &root); err != nil {
		panic(err)
	}
	if err := yamladv.Resolve(&root, dir); err != nil {
		panic(err)
	}

	// Decode the resolved tree into a Go struct.
	type sensor struct {
		Name string `yaml:"name"`
		Unit string `yaml:"unit"`
	}
	type config struct {
		Sensors []sensor `yaml:"sensors"`
	}
	var cfg config
	if err := root.Decode(&cfg); err != nil {
		panic(err)
	}

	fmt.Println(cfg.Sensors[0].Name, cfg.Sensors[0].Unit)
	fmt.Println(cfg.Sensors[1].Name, cfg.Sensors[1].Unit)

	// Output:
	// temperature °C
	// humidity %
}

func ExampleDecoder() {
	// Set up a temporary config dir with an included file.
	dir, _ := os.MkdirTemp("", "yamladv-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(`host: "sensor.example.com"`), 0o644)

	mainYAML := []byte(`server: !include host.yaml`)

	// Use Decoder as a drop-in replacement for yaml.Decoder.
	dec := yamladv.NewDecoder(bytes.NewReader(mainYAML))
	dec.SetBaseDir(dir)

	type server struct {
		Host string `yaml:"host"`
	}
	type config struct {
		Server server `yaml:"server"`
	}
	var cfg config
	if err := dec.Decode(&cfg); err != nil {
		panic(err)
	}

	fmt.Println(cfg.Server.Host)

	// Output:
	// sensor.example.com
}
