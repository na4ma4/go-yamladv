// Package yamladv resolves Home Assistant-style YAML include tags (!include,
// !include_dir_list, !include_dir_named, !include_dir_merge_list,
// !include_dir_merge_named) by walking a yaml.Node tree and splicing in
// content from referenced files and directories.
package yamladv

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	tagInclude              = "!include"
	tagIncludeDirList       = "!include_dir_list"
	tagIncludeDirNamed      = "!include_dir_named"
	tagIncludeDirMergeList  = "!include_dir_merge_list"
	tagIncludeDirMergeNamed = "!include_dir_merge_named"

	tagSeq = "!!seq"
	tagMap = "!!map"
)

// Resolve walks the yaml.Node tree and resolves all include tags.
// baseDir is used to resolve relative paths. Circular includes return an error.
func Resolve(node *yaml.Node, baseDir string) error {
	return resolve(node, baseDir, make(map[string]struct{}))
}

func resolve(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	switch node.Tag {
	case tagInclude:
		return handleInclude(node, baseDir, seen)
	case tagIncludeDirList:
		return handleDirList(node, baseDir, seen)
	case tagIncludeDirNamed:
		return handleDirNamed(node, baseDir, seen)
	case tagIncludeDirMergeList:
		return handleDirMergeList(node, baseDir, seen)
	case tagIncludeDirMergeNamed:
		return handleDirMergeNamed(node, baseDir, seen)
	}

	// Recurse into children
	for i := range node.Content {
		if err := resolve(node.Content[i], baseDir, seen); err != nil {
			return err
		}
	}
	return nil
}

func handleInclude(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	path := resolvePath(baseDir, node.Value)

	var absPath string
	{
		var err error
		absPath, err = filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolving include path %q: %w", path, err)
		}
		if _, ok := seen[absPath]; ok {
			return fmt.Errorf("circular include detected: %s", absPath)
		}
		seen[absPath] = struct{}{}
	}

	var data []byte
	{
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading included file %q: %w", path, err)
		}
	}

	var included yaml.Node
	if err := yaml.Unmarshal(data, &included); err != nil {
		return fmt.Errorf("parsing included file %q: %w", path, err)
	}

	// Unwrap DocumentNode wrapper produced by yaml.Unmarshal
	content := &included
	if included.Kind == yaml.DocumentNode && len(included.Content) > 0 {
		content = included.Content[0]
	}

	// Recurse into the newly loaded node with the included file's dir as baseDir
	newBaseDir := dirOf(path)
	if err := resolve(content, newBaseDir, seen); err != nil {
		return err
	}

	delete(seen, absPath)

	replaceNode(node, content)
	return nil
}

func handleDirList(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	dir := resolvePath(baseDir, node.Value)
	var files []yamlFile
	{
		var err error
		files, err = findYAMLFiles(dir)
		if err != nil {
			return fmt.Errorf("finding yaml files in %q: %w", dir, err)
		}
	}

	// Build a sequence node: each file's content is one list entry.
	// Per Home Assistant spec, each file must contain only one entry.
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}

	for _, f := range files {
		doc, err := parseFileWithIncludes(f, dirOf(f.path), seen)
		if err != nil {
			return err
		}
		if doc == nil {
			continue // skip empty files
		}
		seq.Content = append(seq.Content, doc)
	}

	replaceNode(node, seq)
	return nil
}

func handleDirNamed(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	dir := resolvePath(baseDir, node.Value)

	var files []yamlFile
	{
		var err error
		files, err = findYAMLFiles(dir)
		if err != nil {
			return fmt.Errorf("finding yaml files in %q: %w", dir, err)
		}
	}

	// Build a mapping node: key = filename (sans extension), value = file content
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}

	for _, f := range files {
		doc, err := parseFileWithIncludes(f, dirOf(f.path), seen)
		if err != nil {
			return err
		}
		if doc == nil {
			continue // skip empty files
		}
		key := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: fileStem(f.path),
		}
		mapping.Content = append(mapping.Content, key, doc)
	}

	replaceNode(node, mapping)
	return nil
}

func handleDirMergeList(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	dir := resolvePath(baseDir, node.Value)
	var files []yamlFile
	{
		var err error
		files, err = findYAMLFiles(dir)
		if err != nil {
			return fmt.Errorf("finding yaml files in %q: %w", dir, err)
		}
	}

	// Concatenate all sequence entries into one merged sequence
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: tagSeq}

	for _, f := range files {
		doc, err := parseFileWithIncludes(f, dirOf(f.path), seen)
		if err != nil {
			return err
		}
		if doc == nil {
			continue // skip empty files
		}
		if doc.Kind != yaml.SequenceNode {
			return fmt.Errorf(
				"!include_dir_merge_list file %q must contain a list, got %s",
				f.path, nodeKindName(doc.Kind),
			)
		}
		seq.Content = append(seq.Content, doc.Content...)
	}

	replaceNode(node, seq)
	return nil
}

func handleDirMergeNamed(node *yaml.Node, baseDir string, seen map[string]struct{}) error {
	dir := resolvePath(baseDir, node.Value)
	var files []yamlFile
	{
		var err error
		files, err = findYAMLFiles(dir)
		if err != nil {
			return fmt.Errorf("finding yaml files in %q: %w", dir, err)
		}
	}

	// Merge all mapping entries into one mapping (later files override duplicate keys)
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: tagMap}
	seenKeys := make(map[string]int) // key name → index in mapping.Content (key position)

	for _, f := range files {
		doc, err := parseFileWithIncludes(f, dirOf(f.path), seen)
		if err != nil {
			return err
		}
		if doc == nil {
			continue // skip empty files
		}
		if doc.Kind != yaml.MappingNode {
			return fmt.Errorf(
				"!include_dir_merge_named file %q must contain a mapping, got %s",
				f.path,
				nodeKindName(doc.Kind),
			)
		}
		// Merge key-value pairs; for duplicate keys, replace the existing entry
		for i := 0; i+1 < len(doc.Content); i += 2 {
			keyNode := doc.Content[i]
			valNode := doc.Content[i+1]
			keyName := keyNode.Value
			if prevIdx, exists := seenKeys[keyName]; exists {
				// Replace existing key-value pair
				mapping.Content[prevIdx] = keyNode
				mapping.Content[prevIdx+1] = valNode
			} else {
				// Append new key-value pair
				seenKeys[keyName] = len(mapping.Content)
				mapping.Content = append(mapping.Content, keyNode, valNode)
			}
		}
	}

	replaceNode(node, mapping)
	return nil
}

// yamlFile holds a found YAML file path.
type yamlFile struct {
	path string
}

// findYAMLFiles recursively finds all .yaml and .yml files in a directory,
// sorted alphanumerically by relative path.
func findYAMLFiles(dir string) ([]yamlFile, error) {
	var files []yamlFile

	var entries []os.DirEntry
	{
		var err error
		entries, err = os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading directory %q: %w", dir, err)
		}
	}

	var subdirs []string
	for _, e := range entries {
		name := e.Name()
		fullPath := filepath.Join(dir, name)

		if e.IsDir() {
			subdirs = append(subdirs, fullPath)
			continue
		}

		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			files = append(files, yamlFile{path: fullPath})
		}
	}

	// Sort files in this directory alphanumerically
	sort.Slice(files, func(i, j int) bool {
		return filepath.Base(files[i].path) < filepath.Base(files[j].path)
	})

	// Recurse into subdirectories
	for _, sub := range subdirs {
		subFiles, err := findYAMLFiles(sub)
		if err != nil {
			return nil, err
		}
		files = append(files, subFiles...)
	}

	return files, nil
}

func parseFileWithIncludes(f yamlFile, baseDir string, seen map[string]struct{}) (*yaml.Node, error) {
	var absPath string
	{
		var err error
		absPath, err = filepath.Abs(f.path)
		if err != nil {
			return nil, fmt.Errorf("resolving path %q: %w", f.path, err)
		}
		if _, ok := seen[absPath]; ok {
			return nil, fmt.Errorf("circular include detected: %s", absPath)
		}
		seen[absPath] = struct{}{}
	}

	var data []byte
	{
		var err error
		data, err = os.ReadFile(f.path)
		if err != nil {
			return nil, fmt.Errorf("reading file %q: %w", f.path, err)
		}
	}

	// Skip empty files — they contribute nothing to includes
	//
	//nolint:nilnil // This is intentional: we want to ignore empty files rather than treating them as errors.
	if len(strings.TrimSpace(string(data))) == 0 {
		delete(seen, absPath)
		return nil, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing file %q: %w", f.path, err)
	}

	// yaml.Unmarshal into a Node produces a DocumentNode; the actual content
	// is the first child. Unwrap it.
	content := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		content = doc.Content[0]
	}

	// Skip files that produce no meaningful content (empty, comment-only, or zero-value nodes)
	//
	//nolint:nilnil // This is intentional: we want to ignore empty files rather than treating them as errors.
	if content.Kind != yaml.ScalarNode && content.Kind != yaml.SequenceNode && content.Kind != yaml.MappingNode {
		delete(seen, absPath)
		return nil, nil
	}

	if err := resolve(content, baseDir, seen); err != nil {
		return nil, err
	}

	delete(seen, absPath)

	return content, nil
}

// replaceNode overwrites the target node with the source node.
func replaceNode(target, source *yaml.Node) {
	*target = *source
}

func resolvePath(baseDir, path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return baseDir + "/" + path
}

func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

func fileStem(path string) string {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".yaml") {
		return base[:len(base)-5]
	}
	if strings.HasSuffix(base, ".yml") {
		return base[:len(base)-4]
	}
	return base
}

func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.DocumentNode:
		return "document"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}
