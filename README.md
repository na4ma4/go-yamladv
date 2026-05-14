# go-yamladv

Go library that extends [go.yaml.in/yaml/v3](https://github.com/yaml/go-yaml) with Home Assistant-style YAML include tags for splitting configuration across multiple files and directories.

## Supported Tags

| Tag | Description |
|---|---|
| `!include` | Include a single YAML file |
| `!include_dir_list` | Include all `.yaml`/`.yml` files in a directory as a list (one entry per file) |
| `!include_dir_named` | Include all files in a directory as a mapping keyed by filename (without extension) |
| `!include_dir_merge_list` | Merge all list files in a directory into a single list |
| `!include_dir_merge_named` | Merge all mapping files in a directory into a single mapping (later files override duplicate keys) |

See the [Home Assistant documentation](https://www.home-assistant.io/docs/configuration/splitting_configuration/#advanced-usage) for the original specification of these tags.

## Usage

### `Resolve` — walk a `yaml.Node` tree in-place

```go
var root yaml.Node
yaml.Unmarshal(data, &root)

if err := yamladv.Resolve(&root, "/path/to/config/dir"); err != nil {
    log.Fatal(err)
}

root.Decode(&cfg)
```

### `Decoder` — drop-in replacement for `yaml.Decoder`

```go
f, _ := os.Open("config.yaml")
dec := yamladv.NewDecoder(f)
dec.SetBaseDir("/path/to/config/dir")

var cfg Config
if err := dec.Decode(&cfg); err != nil {
    log.Fatal(err)
}
```

## Install

```bash
go get github.com/na4ma4/go-yamladv
```

## Features

- Recursive includes (included files can include other files)
- Circular include detection with descriptive errors
- Directory scanning is recursive — files in subdirectories are included
- Files are sorted alphanumerically within each directory level
- Empty and comment-only files are silently skipped
- Both `.yaml` and `.yml` extensions are recognised
- Relative paths are resolved relative to `baseDir`; absolute paths are used as-is
