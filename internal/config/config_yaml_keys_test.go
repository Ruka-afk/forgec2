package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestConfigExampleYAMLHasNoDeadKeys fails when the committed config template
// carries a key that no field in the config struct claims.
//
// It deliberately checks config.example.yaml, not config.yaml. The latter is
// gitignored and per-deployment, so it does not exist on a fresh clone or in
// CI — asserting on it would make this test fail everywhere except a working
// copy. config.example.yaml is the tracked contract: it is what CI feeds to
// the smoke and e2e jobs, and what operators copy when they set up a server.
//
// The check matters because the loader uses plain yaml.Unmarshal, which
// ignores unknown keys instead of rejecting them. A dead key is therefore the
// worst kind of configuration: an operator sets it, the server starts
// normally, and nothing happens — with no warning anywhere. A local
// config.yaml had accumulated five of them (server.enable_pprof,
// crypto.force_ecdh, and implant dns_doh_url / dns_dot_addr / dns_ipv6 for
// DNS-over-HTTPS, DNS-over-TLS and IPv6, none of which are implemented) while
// the code that would have read them was removed.
//
// A sub-map whose Go counterpart is a free-form map (malleable headers) has no
// per-key schema and is skipped rather than reported.
func TestConfigExampleYAMLHasNoDeadKeys(t *testing.T) {
	path := filepath.Join("..", "..", "config.example.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var dead []string
	collectDeadKeys("config.example.yaml", raw, reflect.TypeOf(Config{}), &dead)

	if len(dead) > 0 {
		sort.Strings(dead)
		t.Errorf("%s has %d key(s) that no config field reads; yaml.Unmarshal ignores them silently:\n  %s",
			path, len(dead), strings.Join(dead, "\n  "))
		t.Errorf("either wire the key up to a Config field, or delete it from %s. "+
			"If the feature is intentionally a placeholder, leave the key out and say so in code.", path)
	}
}

func collectDeadKeys(path string, node map[string]any, typ reflect.Type, dead *[]string) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return // free-form map/slice: no per-key schema exists
	}
	for key, child := range node {
		field, ok := structFieldForTag(typ, key)
		if !ok {
			*dead = append(*dead, path+"."+key)
			continue
		}
		if sub, isMap := child.(map[string]any); isMap {
			collectDeadKeys(path+"."+key, sub, field.Type, dead)
		}
	}
}

// structFieldForTag finds the struct field a YAML key maps to, honouring the
// yaml tag first and falling back to json, since parts of the config were
// introduced with either.
func structFieldForTag(t reflect.Type, key string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		for _, tagName := range []string{"yaml", "json"} {
			tag := f.Tag.Get(tagName)
			if tag == "" || tag == "-" {
				continue
			}
			if name := strings.Split(tag, ",")[0]; name == key {
				return f, true
			}
		}
	}
	return reflect.StructField{}, false
}
