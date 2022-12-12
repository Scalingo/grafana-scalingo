package codegen

import (
	"bytes"
	"embed"
	"text/template"
	"time"
)

// All the parsed templates in the tmpl subdirectory
var tmpls *template.Template

func init() {
	base := template.New("codegen").Funcs(template.FuncMap{
		"now": time.Now,
	})
	tmpls = template.Must(base.ParseFS(tmplFS, "tmpl/*.tmpl"))
}

//go:embed tmpl/*.tmpl
var tmplFS embed.FS

// The following group of types, beginning with tvars_*, all contain the set
// of variables expected by the corresponding named template file under tmpl/
type (
	tvars_autogen_header struct {
		GeneratorPath  string
		LineagePath    string
		LineageCUEPath string
		GenLicense     bool
	}
	tvars_kind_registry struct {
		// Header tvars_autogen_header
		NumRaw, NumStructured int
		PackageName           string
		KindPackagePrefix     string
		Kinds                 []*DeclForGen
	}
	tvars_coremodel_imports struct {
		PackageName string
	}
	tvars_plugin_lineage_binding struct {
		SlotName               string
		LatestMajv, LatestMinv uint
	}
	tvars_plugin_lineage_file struct {
		PackageName string
		PluginID    string
		PluginType  string
		HasModels   bool
		RootCUE     bool
		SlotImpls   []tvars_plugin_lineage_binding
		Header      tvars_autogen_header
	}
	tvars_plugin_registry struct {
		Header  tvars_autogen_header
		Plugins []struct {
			PkgName    string
			Path       string
			ImportPath string
			NoAlias    bool
		}
	}
)

type HeaderVars = tvars_autogen_header

// GenGrafanaHeader creates standard header elements for generated Grafana files.
func GenGrafanaHeader(vars HeaderVars) string {
	buf := new(bytes.Buffer)
	if err := tmpls.Lookup("autogen_header.tmpl").Execute(buf, vars); err != nil {
		panic(err)
	}
	return buf.String()
}
