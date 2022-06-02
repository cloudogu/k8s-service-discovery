package warp

import (
	"encoding/json"
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"log"

	"sort"

	"github.com/pkg/errors"
)

// ConfigReader reads the configuration for the warp menu from etcd
type ConfigReader struct {
	configuration     *config.Configuration
	registry          registry.WatchConfigurationContext
	doguConverter     DoguConverter
	externalConverter ExternalConverter
}

// DoguConverter is used to read dogus from the registry and convert them to objects fitting in the warp menu
type DoguConverter interface {
	ReadAndUnmarshalDogu(registry registry.WatchConfigurationContext, key string, tag string) (types.EntryWithCategory, error)
}

// ExternalConverter is used to read external links from the registry and convert them to objects fitting in the warp menu
type ExternalConverter interface {
	ReadAndUnmarshalExternal(registry registry.WatchConfigurationContext, key string) (types.EntryWithCategory, error)
}

const disableWarpSupportEntriesConfigurationKey = "/config/_global/disabled_warpmenu_support_entries"

func (reader *ConfigReader) readFromConfig(configuration *config.Configuration) (types.Categories, error) {
	var data types.Categories

	for _, source := range configuration.Sources {
		// Disabled support entries refresh every time
		if source.Type == "disabled_support_entries" {
			continue
		}

		categories, err := reader.readSource(source)
		if err != nil {
			log.Println("Error during read:", err)
		}
		data.InsertCategories(categories)
	}

	log.Println("read SupportEntries")
	disabledSupportEntries, err := reader.getDisabledSupportIdentifiers()
	if err != nil {
		log.Println("Error during support read:", err)
	}
	supportCategory := reader.readSupport(configuration.Support, disabledSupportEntries)
	data.InsertCategories(supportCategory)
	return data, nil
}

func (reader *ConfigReader) readSource(source config.Source) (types.Categories, error) {
	switch source.Type {
	case "dogus":
		return reader.dogusReader(source)
	case "externals":
		return reader.externalsReader(source)
	}
	return nil, errors.Errorf("wrong source type: %v", source.Type)
}

func (reader *ConfigReader) externalsReader(source config.Source) (types.Categories, error) {
	log.Printf("read externals from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read root entry %s from etcd: %w", source.Path, err)
	}
	externals := []types.EntryWithCategory{}
	for _, child := range resp {
		external, err := reader.externalConverter.ReadAndUnmarshalExternal(reader.registry, child)
		if err == nil {
			externals = append(externals, external)
		}
	}
	return reader.createCategories(externals), nil
}

// dogusReader reads from etcd and converts the keys and values to a warp menu
// conform structure
func (reader *ConfigReader) dogusReader(source config.Source) (types.Categories, error) {
	log.Printf("read dogus from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read root entry %s from etcd: %w", source.Path, err)
	}
	dogus := []types.EntryWithCategory{}
	for _, path := range resp {
		dogu, err := reader.doguConverter.ReadAndUnmarshalDogu(reader.registry, path, source.Tag)
		if err == nil && dogu.Entry.Title != "" {
			dogus = append(dogus, dogu)
		}
	}

	return reader.createCategories(dogus), nil
}

func (reader *ConfigReader) getDisabledSupportIdentifiers() ([]string, error) {
	disabledSupportEntries, err := reader.registry.Get(disableWarpSupportEntriesConfigurationKey)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read configuration entry %s from etcd: %w", disableWarpSupportEntriesConfigurationKey, err)
	}

	var disabledEntries []string
	err = json.Unmarshal([]byte(disabledSupportEntries), &disabledEntries)
	if err != nil {
		return []string{}, fmt.Errorf("failed to unmarshal etcd key: %w", err)
	}

	return disabledEntries, nil
}

func (reader *ConfigReader) readSupport(supportSources []config.SupportSource, disabledSupportEntries []string) types.Categories {
	var supportEntries []types.EntryWithCategory

	for _, supportSource := range supportSources {
		// supportSource -> EntryWithCategory
		if !StringInSlice(supportSource.Identifier, disabledSupportEntries) {
			var entry types.Entry
			if supportSource.External {
				entry = types.Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: types.TARGET_EXTERNAL}
			} else {
				entry = types.Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: types.TARGET_SELF}
			}
			entryWithCategory := types.EntryWithCategory{Entry: entry, Category: "Support"}
			supportEntries = append(supportEntries, entryWithCategory)
		}
	}

	return reader.createCategories(supportEntries)
}

func (reader *ConfigReader) createCategories(entries []types.EntryWithCategory) types.Categories {
	categories := map[string]*types.Category{}

	for _, entry := range entries {
		categoryName := entry.Category
		category := categories[categoryName]
		if category == nil {
			category = &types.Category{
				Title:   categoryName,
				Entries: types.Entries{},
				Order:   reader.configuration.Order[categoryName],
			}
			categories[categoryName] = category
		}
		category.Entries = append(category.Entries, entry.Entry)
	}

	result := types.Categories{}
	for _, cat := range categories {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}
	sort.Sort(result)
	return result
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
