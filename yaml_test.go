package yamladv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/na4ma4/go-yamladv"
	"go.yaml.in/yaml/v3"
)

func loadTestCase(t *testing.T, name string) {
	t.Helper()

	// Load the test.yaml and expect.yaml from testdata/<name>/ directory and compare the results after decoding with yamladv and yaml.v3.
	// The test.yaml will contain the input YAML with include tags, and expect.yaml will contain the expected output after resolving includes.
	caseDir := filepath.Join("testdata", name)

	var testData []byte
	{
		var err error
		testData, err = os.ReadFile(filepath.Join(caseDir, "test.yaml"))
		if err != nil {
			t.Fatalf("read test.yaml: %v", err)
		}
	}

	var expectData []byte
	{
		var err error
		expectData, err = os.ReadFile(filepath.Join(caseDir, "expect.yaml"))
		if err != nil {
			t.Fatalf("read expect.yaml: %v", err)
		}
	}

	// t.Chdir(caseDir)

	dec := yamladv.NewDecoder(bytes.NewReader(testData))
	dec.SetBaseDir(caseDir)
	var got any
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("yamladv decode: %v", err)
	}

	var want any
	if err := yaml.Unmarshal(expectData, &want); err != nil {
		t.Fatalf("yaml unmarshal expect: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded output mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestDecoderFixtures(t *testing.T) {
	t.Parallel()

	tests := []string{
		"210292c9-237d-4c97-ad7d-98d1bf48f108",
		"5eede638-8bb2-4d2f-b6a4-2f2e5b146030",
		"6abc0a89-7237-4065-a0da-68e3eda2beea",
		"ff829067-b9e4-45cc-b733-04a79f2ae8fb",
		"diamond-include",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()
			loadTestCase(t, tt)
		})
	}
}
