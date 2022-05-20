package featuremgmt

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
)

func TestFeatureToggleFiles(t *testing.T) {
	// Typescript files
	verifyAndGenerateFile(t,
		"../../../packages/grafana-data/src/types/featureToggles.gen.ts",
		generateTypeScript(),
	)

	// Golang files
	verifyAndGenerateFile(t,
		"toggles_gen.go",
		generateRegistry(t),
	)
}

func verifyAndGenerateFile(t *testing.T, fpath string, gen string) {
	// nolint:gosec
	// We can ignore the gosec G304 warning since this is a test and the function is only called explicitly above
	body, err := ioutil.ReadFile(fpath)
	if err == nil {
		if diff := cmp.Diff(gen, string(body)); diff != "" {
			str := fmt.Sprintf("body mismatch (-want +got):\n%s\n", diff)
			err = fmt.Errorf(str)
		}
	}

	if err != nil {
		e2 := os.WriteFile(fpath, []byte(gen), 0644)
		if e2 != nil {
			t.Errorf("error writing file: %s", e2.Error())
		}
		abs, _ := filepath.Abs(fpath)
		t.Errorf("feature toggle do not match: %s (%s)", err.Error(), abs)
		t.Fail()
	}
}

func generateTypeScript() string {
	buf := `// NOTE: This file was auto generated.  DO NOT EDIT DIRECTLY!
// To change feature flags, edit:
//  pkg/services/featuremgmt/registry.go
// Then run tests in:
//  pkg/services/featuremgmt/toggles_gen_test.go

/**
 * Describes available feature toggles in Grafana. These can be configured via
 * conf/custom.ini to enable features under development or not yet available in
 * stable version.
 *
 * Only enabled values will be returned in this interface
 *
 * @public
 */
export interface FeatureToggles {
  [name: string]: boolean | undefined; // support any string value

`
	for _, flag := range standardFeatureFlags {
		buf += "  " + getTypeScriptKey(flag.Name) + "?: boolean;\n"
	}

	buf += "}\n"
	return buf
}

func getTypeScriptKey(key string) string {
	if strings.Contains(key, "-") || strings.Contains(key, ".") {
		return "['" + key + "']"
	}
	return key
}

func isLetterOrNumber(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c)
}

func asCamelCase(key string) string {
	parts := strings.FieldsFunc(key, isLetterOrNumber)
	for idx, part := range parts {
		parts[idx] = strings.Title(part)
	}
	return strings.Join(parts, "")
}

func generateRegistry(t *testing.T) string {
	tmpl, err := template.New("fn").Parse(`
{{"\t"}}// Flag{{.CamelCase}}{{.Ext}}
{{"\t"}}Flag{{.CamelCase}} = "{{.Flag.Name}}"
`)
	if err != nil {
		t.Fatal("error reading template", "error", err.Error())
		return ""
	}

	data := struct {
		CamelCase string
		Flag      FeatureFlag
		Ext       string
	}{
		CamelCase: "?",
	}

	var buff bytes.Buffer

	buff.WriteString(`// NOTE: This file was auto generated.  DO NOT EDIT DIRECTLY!
// To change feature flags, edit:
//  pkg/services/featuremgmt/registry.go
// Then run tests in:
//  pkg/services/featuremgmt/toggles_gen_test.go

package featuremgmt

const (`)

	for _, flag := range standardFeatureFlags {
		data.CamelCase = asCamelCase(flag.Name)
		data.Flag = flag
		data.Ext = ""

		if flag.Description != "" {
			data.Ext += "\n\t// " + flag.Description
		}

		_ = tmpl.Execute(&buff, data)
	}
	buff.WriteString(")\n")

	return buff.String()
}
