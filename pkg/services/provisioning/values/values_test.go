package values

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/grafana/grafana/pkg/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"
)

func TestValues(t *testing.T) {
	t.Run("Values", func(t *testing.T) {
		err := os.Setenv("INT", "1")
		require.NoError(t, err)
		err = os.Setenv("STRING", "test")
		require.NoError(t, err)
		err = os.Setenv("EMPTYSTRING", "")
		require.NoError(t, err)
		err = os.Setenv("BOOL", "true")
		require.NoError(t, err)

		defer func() {
			err := os.Unsetenv("INT")
			require.NoError(t, err)
			err = os.Unsetenv("STRING")
			require.NoError(t, err)
			err = os.Unsetenv("EMPTYSTRING")
			require.NoError(t, err)
			err = os.Unsetenv("BOOL")
			require.NoError(t, err)
		}()

		t.Run("IntValue", func(t *testing.T) {
			type Data struct {
				Val IntValue `yaml:"val"`
			}
			d := &Data{}

			t.Run("Should unmarshal simple number", func(t *testing.T) {
				unmarshalingTest(t, `val: 1`, d)
				require.Equal(t, d.Val.Value(), 1)
				require.Equal(t, d.Val.Raw, "1")
			})

			t.Run("Should unmarshal env var", func(t *testing.T) {
				unmarshalingTest(t, `val: $INT`, d)
				require.Equal(t, d.Val.Value(), 1)
				require.Equal(t, d.Val.Raw, "$INT")
			})

			t.Run("Should ignore empty value", func(t *testing.T) {
				unmarshalingTest(t, `val: `, d)
				require.Equal(t, d.Val.Value(), 0)
				require.Equal(t, d.Val.Raw, "")
			})
		})

		t.Run("StringValue", func(t *testing.T) {
			type Data struct {
				Val StringValue `yaml:"val"`
			}
			d := &Data{}

			t.Run("Should unmarshal simple string", func(t *testing.T) {
				unmarshalingTest(t, `val: test`, d)
				require.Equal(t, d.Val.Value(), "test")
				require.Equal(t, d.Val.Raw, "test")
			})

			t.Run("Should unmarshal env var", func(t *testing.T) {
				unmarshalingTest(t, `val: $STRING`, d)
				require.Equal(t, d.Val.Value(), "test")
				require.Equal(t, d.Val.Raw, "$STRING")
			})

			t.Run("Should ignore empty value", func(t *testing.T) {
				unmarshalingTest(t, `val: `, d)
				require.Equal(t, d.Val.Value(), "")
				require.Equal(t, d.Val.Raw, "")
			})

			t.Run("empty var should have empty value", func(t *testing.T) {
				unmarshalingTest(t, `val: $EMPTYSTRING`, d)
				require.Equal(t, d.Val.Value(), "")
				require.Equal(t, d.Val.Raw, "$EMPTYSTRING")
			})

			t.Run("$$ should be a literal $", func(t *testing.T) {
				unmarshalingTest(t, `val: $$`, d)
				require.Equal(t, d.Val.Value(), "$")
				require.Equal(t, d.Val.Raw, "$$")
			})

			t.Run("$$ should be a literal $ and not expanded within a string", func(t *testing.T) {
				unmarshalingTest(t, `val: mY,Passwo$$rd`, d)
				require.Equal(t, d.Val.Value(), "mY,Passwo$rd")
				require.Equal(t, d.Val.Raw, "mY,Passwo$$rd")
			})
		})

		t.Run("BoolValue", func(t *testing.T) {
			type Data struct {
				Val BoolValue `yaml:"val"`
			}
			d := &Data{}

			t.Run("Should unmarshal bool value", func(t *testing.T) {
				unmarshalingTest(t, `val: true`, d)
				require.True(t, d.Val.Value())
				require.Equal(t, d.Val.Raw, "true")
			})

			t.Run("Should unmarshal explicit string", func(t *testing.T) {
				unmarshalingTest(t, `val: "true"`, d)
				require.True(t, d.Val.Value())
				require.Equal(t, d.Val.Raw, "true")
			})

			t.Run("Should unmarshal env var", func(t *testing.T) {
				unmarshalingTest(t, `val: $BOOL`, d)
				require.True(t, d.Val.Value())
				require.Equal(t, d.Val.Raw, "$BOOL")
			})

			t.Run("Should ignore empty value", func(t *testing.T) {
				unmarshalingTest(t, `val: `, d)
				require.False(t, d.Val.Value())
				require.Equal(t, d.Val.Raw, "")
			})
		})

		t.Run("JSONValue", func(t *testing.T) {
			type Data struct {
				Val JSONValue `yaml:"val"`
			}
			d := &Data{}

			t.Run("Should unmarshal variable nesting", func(t *testing.T) {
				doc := `
                 val:
                   one: 1
                   two: $STRING
                   three:
                     - 1
                     - two
                     - three:
                         inside: $STRING
                     - six:
                         empty:
                   four:
                     nested:
                       onemore: $INT
                   multiline: >
                     Some text with $STRING
                   anchor: &label $INT
                   anchored: *label
               `
				unmarshalingTest(t, doc, d)

				type stringMap = map[string]interface{}
				require.Equal(t, d.Val.Value(), stringMap{
					"one": 1,
					"two": "test",
					"three": []interface{}{
						1,
						"two",
						stringMap{
							"three": stringMap{
								"inside": "test",
							},
						},
						stringMap{
							"six": stringMap{
								"empty": interface{}(nil),
							},
						},
					},
					"four": stringMap{
						"nested": stringMap{
							"onemore": "1",
						},
					},
					"multiline": "Some text with test\n",
					"anchor":    "1",
					"anchored":  "1",
				})

				require.Equal(t, d.Val.Raw, stringMap{
					"one": 1,
					"two": "$STRING",
					"three": []interface{}{
						1,
						"two",
						stringMap{
							"three": stringMap{
								"inside": "$STRING",
							},
						},
						stringMap{
							"six": stringMap{
								"empty": interface{}(nil),
							},
						},
					},
					"four": stringMap{
						"nested": stringMap{
							"onemore": "$INT",
						},
					},
					"multiline": "Some text with $STRING\n",
					"anchor":    "$INT",
					"anchored":  "$INT",
				})
			})
		})

		t.Run("StringMapValue", func(t *testing.T) {
			type Data struct {
				Val StringMapValue `yaml:"val"`
			}
			d := &Data{}

			t.Run("Should unmarshal mapping", func(t *testing.T) {
				doc := `
                 val:
                   one: 1
                   two: "test string"
                   three: $STRING
                   four: true
               `
				unmarshalingTest(t, doc, d)
				require.Equal(t, d.Val.Value(), map[string]string{
					"one":   "1",
					"two":   "test string",
					"three": "test",
					"four":  "true",
				})

				require.Equal(t, d.Val.Raw, map[string]string{
					"one":   "1",
					"two":   "test string",
					"three": "$STRING",
					"four":  "true",
				})
			})
		})
	})
}

func unmarshalingTest(t *testing.T, document string, out interface{}) {
	err := yaml.Unmarshal([]byte(document), out)
	require.NoError(t, err)
}

func TestValues_readFile(t *testing.T) {
	type Data struct {
		Val StringValue `yaml:"val"`
	}

	f, err := ioutil.TempFile(os.TempDir(), "file expansion *")
	require.NoError(t, err)
	file := f.Name()

	defer func() {
		require.NoError(t, os.Remove(file))
	}()

	const expected = "hello, world"
	_, err = f.WriteString(expected)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	data := &Data{}
	err = yaml.Unmarshal([]byte(fmt.Sprintf("val: $__file{%s}", file)), data)
	require.NoError(t, err)
	assert.Equal(t, expected, data.Val.Value())
}

func TestValues_expanderError(t *testing.T) {
	type Data struct {
		Top JSONValue `yaml:"top"`
	}

	setting.AddExpander("fail", 0, failExpander{})

	data := &Data{}
	err := yaml.Unmarshal([]byte("top:\n  val: $__fail{val}"), data)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, errExpand), "expected error to wrap: %v\ngot: %v", errExpand, err)
	assert.Empty(t, data)
}

var errExpand = errors.New("test error: bad expander")

type failExpander struct{}

func (f failExpander) SetupExpander(file *ini.File) error {
	return nil
}

func (f failExpander) Expand(s string) (string, error) {
	return "", errExpand
}
