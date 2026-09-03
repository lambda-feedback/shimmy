package schema

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

// muEdSpecFS holds every embedded µEd OpenAPI spec. Files are named
// mued_v<version>.yml; adding a new version is a matter of dropping in another
// such file — no Go change is required here.
//
//go:embed mued_v*.yml
var muEdSpecFS embed.FS

// MuEdOpenAPISpecs maps µEd API version -> raw OpenAPI spec bytes, discovered
// from the embedded mued_v<version>.yml files at package load.
var MuEdOpenAPISpecs = mustLoadMuEdSpecs()

// OpenAPISpec is the latest embedded µEd OpenAPI spec, retained for callers that
// still expect a single spec blob.
var OpenAPISpec = MuEdOpenAPISpecs[LatestMuEdSpecVersion()]

func mustLoadMuEdSpecs() map[string][]byte {
	entries, err := muEdSpecFS.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("reading embedded µEd specs: %v", err))
	}

	out := make(map[string][]byte)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "mued_v") || !strings.HasSuffix(name, ".yml") {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(name, "mued_v"), ".yml")
		data, err := muEdSpecFS.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("reading embedded µEd spec %s: %v", name, err))
		}
		out[version] = data
	}

	if len(out) == 0 {
		panic("no embedded µEd OpenAPI specs found")
	}
	return out
}

// MuEdSpecVersions returns the embedded spec versions in ascending order.
// Ordering is lexical, which is sufficient while versions stay single-digit;
// revisit if a component ever reaches double digits.
//
// The lexical sort also treats a pre-release tag as newer than its base
// release: "0.1.1-dev" sorts after "0.1.0", so the dev spec is the "latest"
// and drives the single-spec callers (LoadOpenAPISpec, MuEdHandler.Spec) that
// need its SSE schemas. Note a future real "0.1.1" would sort *before*
// "0.1.1-dev" — revisit this ordering when the dev tag is promoted.
func MuEdSpecVersions() []string {
	versions := make([]string, 0, len(MuEdOpenAPISpecs))
	for v := range MuEdOpenAPISpecs {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return versions
}

// LatestMuEdSpecVersion returns the highest embedded spec version.
func LatestMuEdSpecVersion() string {
	versions := MuEdSpecVersions()
	return versions[len(versions)-1]
}
