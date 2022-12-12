//go:build ignore
// +build ignore

//go:generate go run gen.go

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/errors"
	"github.com/grafana/codejen"
	"github.com/grafana/grafana/pkg/codegen"
	"github.com/grafana/grafana/pkg/cuectx"
	"github.com/grafana/grafana/pkg/kindsys"
)

const sep = string(filepath.Separator)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "plugin thema code generator does not currently accept any arguments\n, got %q", os.Args)
		os.Exit(1)
	}

	// Core kinds composite code generator. Produces all generated code in
	// grafana/grafana that derives from raw and structured core kinds.
	coreKindsGen := codejen.JennyListWithNamer[*codegen.DeclForGen](func(decl *codegen.DeclForGen) string {
		return decl.Meta.Common().MachineName
	})

	// All the jennies that comprise the core kinds generator pipeline
	coreKindsGen.Append(
		codegen.GoTypesJenny(kindsys.GoCoreKindParentPath, nil),
		codegen.CoreStructuredKindJenny(kindsys.GoCoreKindParentPath, nil),
		codegen.RawKindJenny(kindsys.GoCoreKindParentPath, nil),
		codegen.BaseCoreRegistryJenny(filepath.Join("pkg", "registry", "corekind"), kindsys.GoCoreKindParentPath),
		codegen.TSTypesJenny(kindsys.TSCoreKindParentPath, &codegen.TSTypesGeneratorConfig{
			GenDirName: func(decl *codegen.DeclForGen) string {
				// FIXME this hardcodes always generating to experimental dir. OK for now, but need generator fanout
				return filepath.Join(decl.Meta.Common().MachineName, "x")
			},
		}),
		codegen.TSVeneerIndexJenny(filepath.Join("packages", "grafana-schema", "src")),
	)

	coreKindsGen.AddPostprocessors(codegen.SlashHeaderMapper("kinds/gen.go"))

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get working directory: %s", err)
		os.Exit(1)
	}
	grootp := strings.Split(cwd, sep)
	groot := filepath.Join(sep, filepath.Join(grootp[:len(grootp)-1]...))

	rt := cuectx.GrafanaThemaRuntime()
	var all []*codegen.DeclForGen

	// structured kinddirs first
	f := os.DirFS(filepath.Join(groot, kindsys.CoreStructuredDeclParentPath))
	kinddirs := elsedie(fs.ReadDir(f, "."))("error reading structured fs root directory")
	for _, ent := range kinddirs {
		if !ent.IsDir() {
			continue
		}
		rel := filepath.Join(kindsys.CoreStructuredDeclParentPath, ent.Name())
		decl, err := kindsys.LoadCoreKind[kindsys.CoreStructuredMeta](rel, rt.Context(), nil)
		if err != nil {
			die(fmt.Errorf("%s is not a valid kind: %s", rel, errors.Details(err, nil)))
		}
		if decl.Meta.MachineName != ent.Name() {
			die(fmt.Errorf("%s: kind's machine name (%s) must equal parent dir name (%s)", rel, decl.Meta.Name, ent.Name()))
		}

		all = append(all, elsedie(codegen.ForGen(rt, decl.Some()))(rel))
	}

	// now raw kinddirs
	f = os.DirFS(filepath.Join(groot, kindsys.RawDeclParentPath))
	kinddirs = elsedie(fs.ReadDir(f, "."))("error reading raw fs root directory")
	for _, ent := range kinddirs {
		if !ent.IsDir() {
			continue
		}
		rel := filepath.Join(kindsys.RawDeclParentPath, ent.Name())
		decl, err := kindsys.LoadCoreKind[kindsys.RawMeta](rel, rt.Context(), nil)
		if err != nil {
			die(fmt.Errorf("%s is not a valid kind: %s", rel, errors.Details(err, nil)))
		}
		if decl.Meta.MachineName != ent.Name() {
			die(fmt.Errorf("%s: kind's machine name (%s) must equal parent dir name (%s)", rel, decl.Meta.Name, ent.Name()))
		}
		dfg, _ := codegen.ForGen(nil, decl.Some())
		all = append(all, dfg)
	}

	sort.Slice(all, func(i, j int) bool {
		return nameFor(all[i].Meta) < nameFor(all[j].Meta)
	})

	jfs, err := coreKindsGen.GenerateFS(all)
	if err != nil {
		die(fmt.Errorf("core kinddirs codegen failed: %w", err))
	}

	if _, set := os.LookupEnv("CODEGEN_VERIFY"); set {
		if err = jfs.Verify(context.Background(), groot); err != nil {
			die(fmt.Errorf("generated code is out of sync with inputs:\n%s\nrun `make gen-cue` to regenerate", err))
		}
	} else if err = jfs.Write(context.Background(), groot); err != nil {
		die(fmt.Errorf("error while writing generated code to disk:\n%s", err))
	}
}

func nameFor(m kindsys.SomeKindMeta) string {
	switch x := m.(type) {
	case kindsys.RawMeta:
		return x.Name
	case kindsys.CoreStructuredMeta:
		return x.Name
	case kindsys.CustomStructuredMeta:
		return x.Name
	case kindsys.ComposableMeta:
		return x.Name
	default:
		// unreachable so long as all the possibilities in KindMetas have switch branches
		panic("unreachable")
	}
}

func elsedie[T any](t T, err error) func(msg string) T {
	if err != nil {
		return func(msg string) T {
			fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err)
			os.Exit(1)
			return t
		}
	}
	return func(msg string) T {
		return t
	}
}

func die(err error) {
	fmt.Fprint(os.Stderr, err, "\n")
	os.Exit(1)
}
