package conf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	lib "github.com/aerospike/aerospike-management-lib"
	"github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
)

func NewASConfigFromBytes(log logr.Logger, src []byte, srcFmt Format) (*asconfig.AsConfig, error) {
	var err error
	var cfg *asconfig.AsConfig

	switch srcFmt {
	case YAML:
		cfg, err = loadYAML(log, src)
	case AsConfig:
		cfg, err = loadAsConf(log, src)
	default:
		return nil, fmt.Errorf("%w %s", ErrInvalidFormat, srcFmt)
	}

	if err != nil {
		return nil, err
	}

	// recreate the management lib config
	// with a sorted config map so that output
	// is always in the same order
	cmap := cfg.ToMap()

	mutateMap(*cmap, []mapping{
		sortLists,
	})

	cfg, err = asconfig.NewMapAsConfig(
		log,
		*cmap,
	)

	return cfg, err
}

func loadYAML(log logr.Logger, src []byte) (*asconfig.AsConfig, error) {

	var data map[string]any

	err := yaml.Unmarshal(src, &data)
	if err != nil {
		return nil, err
	}

	c, err := asconfig.NewMapAsConfig(
		log,
		data,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize asconfig from yaml: %w", err)
	}

	return c, nil
}

func loadAsConf(log logr.Logger, src []byte) (*asconfig.AsConfig, error) {

	reader := bytes.NewReader(src)

	// TODO: Why doesn't the management lib do the map mutation? FromConfFile
	// implies it does.
	c, err := asconfig.FromConfFile(log, "", reader) // TODO: Remove "" when management lib merges PR #41
	if err != nil {
		return nil, fmt.Errorf("failed to parse asconfig file: %w", err)
	}

	// the aerospike management lib parses asconfig files into
	// a format that its validation rejects
	// this is because the schema files are meant to
	// validate the aerospike kubernetes operator's asconfig yaml format
	// so we modify the map here to match that format
	cmap := *c.ToMap()

	mutateMap(cmap, []mapping{
		typedContextsToObject,
		toPlural,
	})

	c, err = asconfig.NewMapAsConfig(
		log,
		cmap,
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

type configMap = lib.Stats

// mapping functions get mapped to each key value pair in a management lib Stats map
// m is the map that k and v came from
type mapping func(k string, v any, m configMap)

// mutateMap maps functions to each key value pair in the management lib's Stats map
// the functions are applied sequentially to each k,v pair.
func mutateMap(in configMap, funcs []mapping) {

	for k, v := range in {

		switch v := v.(type) {
		case configMap:
			mutateMap(v, funcs)
		case []configMap:
			for _, lv := range v {
				mutateMap(lv, funcs)
			}
		}

		for _, f := range funcs {
			f(k, in[k], in)
		}
	}
}

/*
typedContextsToObject converts config entries that the management lib
parses as literal strings into the objects that the yaml schemas expect.
NOTE: As of server 7.0 a context is required for storage-engine memory
so it will no longer be a string. This is still needed for compatibility
with older servers.
Ex configMap

	configMap{
		"storage-engine": "memory"
	}

->

	configMap{
		"storage-engine": configMap{
			"type": "memory"
		}
	}
*/
func typedContextsToObject(k string, v any, m configMap) {

	if isTypedContext(k) {
		v := m[k]
		// if a typed context does not have a map value.
		// then it's value is a string like "memory" or "flash"
		// in order to make valid asconfig yaml we convert this context
		// to a map where "type" maps to the value
		if _, ok := v.(configMap); !ok {
			m[k] = configMap{"type": v}
		}
	}
}

// isTypedContext returns true for asconfig contexts
// that can map to strings instead of contexts
func isTypedContext(in string) bool {

	switch in {
	case "storage-engine", "index-type", "sindex-type":
		return true
	default:
		return false
	}
}

/*
toPlural converts the keys that the management lib asconf parser
parses as singular, to the plural keys that the yaml schemas expect
Ex configMap

	configMap{
		"namespace": []configMap{
			...
		}
	}

->

	configMap{
		"namespaces": []configMap{
			...
		}
	}
*/
func toPlural(k string, v any, m configMap) {

	// convert asconfig fields/contexts that need to be plural
	// in order to create valid asconfig yaml.
	if plural := asconfig.PluralOf(k); plural != k {
		// if the config item can be plural or singular and is not a slice
		// then the item should not be converted to the plural form.
		// If the management lib ever parses list entries as anything other
		// than []string this might have to change.
		if isListOrString(k) {
			if _, ok := v.([]string); !ok {
				return
			}

			if len(v.([]string)) == 1 {
				// the management lib parses all config fields
				// that are in singularToPlural as lists. If these
				// fields are actually scalars then overwrite the list
				// with the single value
				m[k] = v.([]string)[0]
				return
			}
		}

		delete(m, k)
		m[plural] = v
	}
}

// isListOrString returns true for special config fields that may be a
// single string value or a list with multiple strings in the schema files
// NOTE: any time the schema changes to make a value
// a string or a list (array) that value needs to be added here
func isListOrString(name string) bool {
	switch name {
	case "feature-key-file", "tls-authenticate-client":
		return true
	default:
		return false
	}
}

/*
sortLists sorts slices of config sections by the "name" or "type"
key that the management lib adds to config list items
Ex config:
namespace ns2 {}
namespace ns1 {}
->
namespace ns1 {}
namespace ns2 {}

Ex matching configMap

	configMap{
		"namespace": []configMap{
			configMap{
				"name": "ns2",
			},
			configMap{
				"name": "ns1",
			},
		}
	}

->

	configMap{
		"namespace": []configMap{
			configMap{
				"name": "ns1",
			},
			configMap{
				"name": "ns2",
			},
		}
	}
*/
func sortLists(k string, v any, m configMap) {
	if v, ok := v.([]configMap); ok {
		sort.Slice(v, func(i int, j int) bool {
			iv, iok := v[i]["name"]
			jv, jok := v[j]["name"]

			// sections may also use the "type" field to identify themselves
			if !iok {
				iv, iok = v[i]["type"]
			}

			if !jok {
				jv, jok = v[j]["type"]
			}

			// if i or both don't have id fields, consider them i >= j
			if !iok {
				return false
			}

			// if only j has an id field consider i < j
			if !jok {
				return true
			}

			iname := iv.(string)
			jname := jv.(string)

			gt := strings.Compare(iname, jname)

			switch gt {
			case 1:
				return true
			case -1, 0:
				return false
			default:
				panic("unexpected gt value")
			}
		})
		m[k] = v
	}
}
