package main

import (
	"flag"
	"log"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	"github.com/nikpivkin/regoref"
	"github.com/nikpivkin/regoref/internal/metadata"
)

var path = flag.String("path", "", "dir or file")
var overwrite = flag.Bool("overwrite", false, "overwrite the original source file")

func main() {
	flag.Parse()

	if *path == "" {
		log.Fatal("\"path\" is required")
	}

	if err := regoref.Run(
		*path, *overwrite, os.Stdout,
		transformPruneEmptyValues,
		transformCleanupAliases,
		transformSortAliases,
	); err != nil {
		log.Fatal(err)
	}

	log.Println("✅ Formatting completed")
}

func transformPruneEmptyValues(_ string, om *regoref.OrderedMap) {
	pruneEmptyValues(om)
}

func transformCleanupAliases(_ string, om *regoref.OrderedMap) {
	customMap, exists := regoref.GetFromMap[*regoref.OrderedMap](om, "custom")
	if !exists {
		return
	}
	var custom metadata.CustomMetadata
	if err := mapstructure.Decode(customMap.Unwrap(), &custom); err != nil {
		panic(err)
	}

	for i, alias := range custom.Aliases {
		custom.Aliases[i] = strings.TrimSuffix(alias, ".")
	}

	customMap.Set("aliases", custom.Aliases)
	om.Set("custom", customMap)
}

func transformSortAliases(_ string, om *regoref.OrderedMap) {
	customMap, exists := regoref.GetFromMap[*regoref.OrderedMap](om, "custom")
	if !exists {
		return
	}
	var custom metadata.CustomMetadata
	if err := mapstructure.Decode(customMap.Unwrap(), &custom); err != nil {
		panic(err)
	}
	sortAliases(custom.Aliases)
	customMap.Set("aliases", custom.Aliases)
	om.Set("custom", customMap)
}

func pruneEmptyValues(om *regoref.OrderedMap) {
	for k, v := range om.Iter() {
		switch vv := v.(type) {
		case nil:
			om.Remove(k)
		case []any:
			if len(vv) == 0 {
				om.Remove(k)
			}
		case *regoref.OrderedMap:
			pruneEmptyValues(vv)
		}
	}
}

var upperRe = regexp.MustCompile(`^[A-Z0-9]+$`)

func sortAliases(xs []string) {
	slices.SortFunc(xs, func(a, b string) int {
		aAVD := hasAVDPrefix(a)
		bAVD := hasAVDPrefix(b)
		if aAVD != bAVD {
			if aAVD {
				return -1
			}
			return 1
		}

		aUpper := isUpperOrDigits(a)
		bUpper := isUpperOrDigits(b)
		if aUpper != bUpper {
			if aUpper {
				return -1
			}
			return 1
		}

		if len(a) != len(b) {
			return len(a) - len(b)
		}

		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	})
}

func hasAVDPrefix(s string) bool {
	return len(s) >= 4 && s[:4] == "AVD-"
}

func isUpperOrDigits(s string) bool {
	return upperRe.MatchString(s)
}
