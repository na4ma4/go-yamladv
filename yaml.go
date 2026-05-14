package yamladv

import (
	"io"

	"go.yaml.in/yaml/v3"
)

// A Decoder reads and decodes YAML values from an input stream.
type Decoder struct {
	parser  *yaml.Decoder
	baseDir string
}

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may read
// data from r beyond the YAML values requested.
//
// NOTE: The caller should call SetBaseDir to set the base directory
// for resolving include tags before calling Decode.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		parser:  yaml.NewDecoder(r),
		baseDir: ".",
	}
}

func (dec *Decoder) SetBaseDir(dir string) {
	dec.baseDir = dir
}

// KnownFields ensures that the keys in decoded mappings to
// exist as fields in the struct being decoded into.
func (dec *Decoder) KnownFields(enable bool) {
	dec.parser.KnownFields(enable)
}

// Decode wraps the yaml.Decoder's Decode method to use yamladv to resolve
// include tags. The value v must be a pointer to a struct, map, or slice.
func (dec *Decoder) Decode(v any) error {
	var root yaml.Node
	if err := dec.parser.Decode(&root); err != nil {
		return err
	}

	if err := Resolve(&root, dec.baseDir); err != nil {
		return err
	}

	return root.Decode(v)
}
