package asconf

import (
	lib "github.com/aerospike/aerospike-management-lib"
)

// mapping functions get mapped to each key value pair in a management lib Stats map
// m is the map that k and v came from
type mapping func(k string, v any, m lib.Stats)

// mapToStats maps functions to each key value pair in the management lib's Stats map
// the functions are applied sequentially to each k,v pair.
func mapToStats(in lib.Stats, funcs []mapping) {

	for k, v := range in {

		switch v := v.(type) {
		case lib.Stats:
			mapToStats(v, funcs)
		case []lib.Stats:
			for _, lv := range v {
				mapToStats(lv, funcs)
			}
		}

		for _, f := range funcs {
			f(k, v, in)
		}
	}
}

func typedContextsToObject(k string, v any, m lib.Stats) {

	if isTypedContext(k) {
		v := m[k]
		// if a typed context does not have a map value.
		// then it's value is a string like "memory" or "flash"
		// in order to make valid asconfig yaml we convert this context
		// to a map where "type" maps to the value
		if _, ok := v.(lib.Stats); !ok {
			m[k] = map[string]any{"type": v}
		}
	}
}

func toPlural(k string, v any, m lib.Stats) {

	// convert asconfig fields/contexts that need to be plural
	// in order to create valid asconfig yaml.
	if plural, ok := singularToPlural[k]; ok {
		m[plural] = v
		delete(m, k)
	}
}

// isTypedContext returns true for asconfig contexts
// that can map to strings instead of contexts
func isTypedContext(in string) bool {

	switch in {
	case "storage-engine", "index-type":
		return true

	default:
		return false
	}
}

// func isListField(in string) bool {

// 	if _, ok := singularToPlural[in]; ok {
// 		return true
// 	}

// 	return false
// }

// func isListContext(in string) bool {

// 	switch in {
// 	// copied from management lib's isListSection()
// 	case "namespace", "datacenter", "dc", "set", "tls", "file":
// 		return true

// 	default:
// 		return false
// 	}
// }

// copied from the management lib asconfig package
var singularToPlural = map[string]string{
	"access-address":               "access-addresses",
	"address":                      "addresses",
	"alternate-access-address":     "alternate-access-addresses",
	"datacenter":                   "datacenters",
	"dc":                           "dcs",
	"dc-int-ext-ipmap":             "dc-int-ext-ipmap",
	"dc-node-address-port":         "dc-node-address-ports",
	"device":                       "devices",
	"file":                         "files",
	"feature-key-file":             "feature-key-files",
	"mount":                        "mounts",
	"http-url":                     "http-urls",
	"ignore-bin":                   "ignore-bins",
	"ignore-set":                   "ignore-sets",
	"logging":                      "logging",
	"mesh-seed-address-port":       "mesh-seed-address-ports",
	"multicast-group":              "multicast-groups",
	"namespace":                    "namespaces",
	"node-address-port":            "node-address-ports",
	"report-data-op":               "report-data-op",
	"role-query-pattern":           "role-query-patterns",
	"set":                          "sets",
	"ship-bin":                     "ship-bins",
	"ship-set":                     "ship-sets",
	"tls":                          "tls",
	"tls-access-address":           "tls-access-addresses",
	"tls-address":                  "tls-addresses",
	"tls-alternate-access-address": "tls-alternate-access-addresses",
	"tls-mesh-seed-address-port":   "tls-mesh-seed-address-ports",
	"tls-node":                     "tls-nodes",
	"xdr-remote-datacenter":        "xdr-remote-datacenters",
	"tls-authenticate-client":      "tls-authenticate-clients",
}
