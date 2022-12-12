// Package cuectx provides a single, central ["cuelang.org/go/cue".Context] and
// ["github.com/grafana/thema".Runtime] that can be used uniformly across
// Grafana, and related helper functions for loading Thema lineages.

package cuectx

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing/fstest"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"github.com/grafana/grafana"
	"github.com/grafana/thema"
	"github.com/grafana/thema/load"
	"github.com/grafana/thema/vmux"
	"github.com/yalue/merged_fs"
)

var ctx = cuecontext.New()
var rt = thema.NewRuntime(ctx)

// GrafanaCUEContext returns Grafana's singleton instance of [cue.Context].
//
// All code within grafana/grafana that needs a *cue.Context should get it
// from this function, when one was not otherwise provided.
func GrafanaCUEContext() *cue.Context {
	return ctx
}

// GrafanaThemaRuntime returns Grafana's singleton instance of [thema.Runtime].
//
// All code within grafana/grafana that needs a *thema.Runtime should get it
// from this function, when one was not otherwise provided.
func GrafanaThemaRuntime() *thema.Runtime {
	return rt
}

// JSONtoCUE attempts to decode the given []byte into a cue.Value, relying on
// the central Grafana cue.Context provided in this package.
//
// The provided path argument determines the name given to the input bytes if
// later CUE operations (e.g. Thema validation) produce errors related to the
// returned cue.Value.
//
// This is a convenience function for one-off JSON decoding. It's wasteful to
// call it repeatedly. Most use cases should probably prefer making
// their own Thema/CUE decoders.
func JSONtoCUE(path string, b []byte) (cue.Value, error) {
	return vmux.NewJSONCodec(path).Decode(ctx, b)
}

// LoadGrafanaInstancesWithThema loads CUE files containing a lineage
// representing some Grafana core model schema. It is expected to be used when
// implementing a thema.LineageFactory.
//
// This function primarily juggles paths to make CUE's loader happy. Provide the
// path from the grafana root to the directory containing the lineage.cue. The
// lineage.cue file must be the sole contents of the provided fs.FS.
//
// More details on underlying behavior can be found in the docs for github.com/grafana/thema/load.InstanceWithThema.
//
// TODO this approach is complicated and confusing, refactor to something understandable
func LoadGrafanaInstancesWithThema(path string, cueFS fs.FS, rt *thema.Runtime, opts ...thema.BindOption) (thema.Lineage, error) {
	prefix := filepath.FromSlash(path)
	fs, err := prefixWithGrafanaCUE(prefix, cueFS)
	if err != nil {
		return nil, err
	}
	inst, err := load.InstanceWithThema(fs, prefix)

	// Need to trick loading by creating the embedded file and
	// making it look like a module in the root dir.
	if err != nil {
		return nil, err
	}

	val := rt.Context().BuildInstance(inst)

	lin, err := thema.BindLineage(val, rt, opts...)
	if err != nil {
		return nil, err
	}

	return lin, nil
}

// prefixWithGrafanaCUE constructs an fs.FS that merges the provided fs.FS with
// the embedded FS containing Grafana's core CUE files, [grafana.CueSchemaFS].
// The provided prefix should be the relative path from the grafana repository
// root to the directory root of the provided inputfs.
//
// The returned fs.FS is suitable for passing to a CUE loader, such as [load.InstanceWithThema].
func prefixWithGrafanaCUE(prefix string, inputfs fs.FS) (fs.FS, error) {
	m := fstest.MapFS{
		// fstest can recognize only forward slashes.
		// filepath.ToSlash(filepath.Join("cue.mod", "module.cue")): &fstest.MapFile{Data: []byte(`module: "github.com/grafana/grafana"`)},
	}

	prefix = filepath.FromSlash(prefix)
	err := fs.WalkDir(inputfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		b, err := fs.ReadFile(inputfs, path)
		if err != nil {
			return err
		}
		// fstest can recognize only forward slashes.
		m[filepath.ToSlash(filepath.Join(prefix, path))] = &fstest.MapFile{Data: b}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return merged_fs.NewMergedFS(m, grafana.CueSchemaFS), nil
}

// LoadGrafanaInstance wraps [load.InstanceWithThema] to load a
// [*build.Instance] corresponding to a particular path within the
// github.com/grafana/grafana CUE module.
//
// This allows resolution of imports within the grafana or thema CUE modules to
// work correctly and consistently by relying on the embedded FS at
// [grafana.CueSchemaFS] and [thema.CueFS].
//
// relpath should be a relative path path within [grafana.CueSchemaFS] to be
// loaded. Optionally, the caller may provide an additional fs.FS via the
// overlay parameter, which will be merged with [grafana.CueSchemaFS] at
// relpath, and loaded.
//
// pkg, if non-empty, is set as the value of
// ["cuelang.org/go/cue/load".Config.Package]. If the CUE package to be loaded
// is the same as the parent directory name, it should be omitted.
//
// NOTE this function will be removed in favor of a more generic loader
func LoadGrafanaInstance(relpath string, pkg string, overlay fs.FS) (*build.Instance, error) {
	// notes about how this crap needs to work
	//
	// Within grafana/grafana, need:
	// - pass in an fs.FS that, in its root, contains the .cue files to load
	// - has no cue.mod
	// - gets prefixed with the appropriate path within grafana/grafana
	// - and merged with all the other .cue files from grafana/grafana
	// notes about how this crap needs to work
	//
	// Need a prefixing instance loader that:
	//  - can take multiple fs.FS, each one representing a CUE module (nesting?)
	//  - reconcile at most one of the provided fs with cwd
	//    - behavior must differ depending on whether cwd is in a cue module
	//    - behavior should(?) be controllable depending on
	relpath = filepath.ToSlash(relpath)

	var f fs.FS = grafana.CueSchemaFS
	var err error
	if overlay != nil {
		f, err = prefixWithGrafanaCUE(relpath, overlay)
		if err != nil {
			return nil, err
		}
	}

	if pkg != "" {
		return load.InstanceWithThema(f, relpath, load.Package(pkg))
	}
	return load.InstanceWithThema(f, relpath)
}

// BuildGrafanaInstance wraps [LoadGrafanaInstance], additionally building
// the returned [*build.Instance], if valid, into a [cue.Value] that is checked
// for errors before returning.
//
// NOTE this function will be removed in favor of a more generic loader
func BuildGrafanaInstance(ctx *cue.Context, relpath string, pkg string, overlay fs.FS) (cue.Value, error) {
	bi, err := LoadGrafanaInstance(relpath, pkg, overlay)
	if err != nil {
		return cue.Value{}, err
	}

	if ctx == nil {
		ctx = GrafanaCUEContext()
	}
	v := ctx.BuildInstance(bi)
	if v.Err() != nil {
		return v, fmt.Errorf("%s not a valid CUE instance: %w", relpath, v.Err())
	}
	return v, nil
}
