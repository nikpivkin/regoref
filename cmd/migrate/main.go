package main

import (
	"flag"
	"log"
	"maps"
	"os"
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
		transformAvd2Id,
	); err != nil {
		log.Fatal(err)
	}

	log.Println("✅ Migration completed")
}

func transformAvd2Id(filePath string, om *regoref.OrderedMap) {
	// skip tests
	if strings.HasSuffix(filePath, "_test.rego") {
		return
	}

	customMap, exists := regoref.GetFromMap[*regoref.OrderedMap](om, "custom")
	// skip files without metadata
	if !exists {
		return
	}

	var custom metadata.CustomMetadata
	if err := mapstructure.Decode(customMap.Unwrap(), &custom); err != nil {
		panic(err)
	}

	// metadata already migrated
	if custom.AVDID == "" && custom.ShortCode == "" {
		return
	}

	if custom.Provider == "" {
		switch {
		case strings.Contains(filePath, "checks/docker"):
			custom.Provider = "docker"
		case strings.Contains(filePath, "checks/kubernetes"):
			custom.Provider = "kubernetes"
		}
	}

	newID := strings.TrimPrefix(custom.AVDID, "AVD-")
	customMap.Set("id", newID)

	longID := buildLongID(custom)
	customMap.Set("long_id", longID)

	newAliases := rebuildAliases(custom, newID)
	customMap.Set("aliases", newAliases)

	customMap.MoveAfter("id", "long_id")
	customMap.MoveAfter("long_id", "aliases")

	customMap.Remove("avd_id")
	customMap.Remove("short_code")

	om.Set("custom", customMap)
}

func buildLongID(custom metadata.CustomMetadata) string {
	service := strings.ReplaceAll(custom.Service, "-", "")
	shortCode := strings.ReplaceAll(custom.ShortCode, "_", "-")

	parts := []string{custom.Provider}
	if custom.Service != "" {
		parts = append(parts, service)
	}
	parts = append(parts, shortCode)
	return strings.Join(parts, "-")
}

func rebuildAliases(custom metadata.CustomMetadata, newID string) []string {
	set := map[string]struct{}{
		custom.ID:        {},
		custom.AVDID:     {},
		custom.ShortCode: {},
	}

	for _, alias := range custom.Aliases {
		set[alias] = struct{}{}
	}

	delete(set, newID)
	return slices.Collect(maps.Keys(set))
}
