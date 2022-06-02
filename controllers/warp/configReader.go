package warp

import (
	"encoding/json"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"

	"sort"

	"github.com/pkg/errors"
)

// ConfigReader reads the configuration for the warp menu from etcd
type ConfigReader struct {
	configuration *config.Configuration
	registry      registry.WatchConfigurationContext
}

const disableWarpSupportEntriesConfigurationKey = "/config/_global/disabled_warpmenu_support_entries"

func (reader *ConfigReader) createCategories(entries []EntryWithCategory) Categories {
	categories := map[string]*Category{}

	for _, entry := range entries {
		categoryName := entry.Category
		category := categories[categoryName]
		if category == nil {
			category = &Category{
				Title:   categoryName,
				Entries: Entries{},
				Order:   reader.configuration.Order[categoryName],
			}
			categories[categoryName] = category
		}
		category.Entries = append(category.Entries, entry.Entry)
	}

	result := Categories{}
	for _, cat := range categories {
		sort.Sort(cat.Entries)
		result = append(result, cat)
	}
	sort.Sort(result)
	return result
}

// dogusReader reads from etcd and converts the keys and values to a warp menu
// conform structure
func (reader *ConfigReader) dogusReader(source config.Source) (Categories, error) {
	ctrl.Log.Info("read dogus from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read root entry %s from etcd: %w", source.Path, err)
	}
	dogus := []EntryWithCategory{}
	for _, child := range resp {
		dogu, err := readAndUnmarshalDogu(reader.registry, child, source.Tag)
		if err == nil && dogu.Entry.Title != "" {
			dogus = append(dogus, dogu)
		}
	}

	return reader.createCategories(dogus), nil
}

func (reader *ConfigReader) externalsReader(source config.Source) (Categories, error) {
	ctrl.Log.Info("read externals from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read root entry %s from etcd: %w", source.Path, err)
	}
	externals := []EntryWithCategory{}
	for _, child := range resp {
		external, err := readAndUnmarshalExternal(reader.registry, child)
		if err == nil {
			externals = append(externals, external)
		}
	}
	return reader.createCategories(externals), nil
}

func (reader *ConfigReader) readSource(source config.Source) (Categories, error) {
	switch source.Type {
	case "dogus":
		return reader.dogusReader(source)
	case "externals":
		return reader.externalsReader(source)
	}
	return nil, errors.Errorf("wrong source type: %v", source.Type)
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

func (reader *ConfigReader) readSupport(supportSources []config.SupportSource, disabledSupportEntries []string) (Categories, error) {
	var supportEntries []EntryWithCategory

	for _, supportSource := range supportSources {
		// supportSource -> EntryWithCategory
		if !StringInSlice(supportSource.Identifier, disabledSupportEntries) {
			var entry Entry
			if supportSource.External {
				entry = Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: TARGET_EXTERNAL}
			} else {
				entry = Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: TARGET_SELF}
			}
			entryWithCategory := EntryWithCategory{Entry: entry, Category: "Support"}
			supportEntries = append(supportEntries, entryWithCategory)
		}
	}

	return reader.createCategories(supportEntries), nil
}

func (reader *ConfigReader) readFromConfig(configuration *config.Configuration) (Categories, error) {
	var data Categories

	for _, source := range configuration.Sources {
		// Disabled support entries refresh every time
		if source.Type == "disabled_support_entries" {
			continue
		}

		categories, err := reader.readSource(source)
		if err != nil {
			ctrl.Log.Info("Error during read:", err)
		}
		data.insertCategories(categories)
	}

	ctrl.Log.Info("read SupportEntries")
	disabledSupportEntries, err := reader.getDisabledSupportIdentifiers()
	if err != nil {
		ctrl.Log.Info("failed to get disabled support identifiers:", err)
	}

	supportCategory, err := reader.readSupport(configuration.Support, disabledSupportEntries)
	if err != nil {
		ctrl.Log.Info("Error during support read:", err)
	}
	if supportCategory.Len() == 0 {
		ctrl.Log.Info("support Category is empty, no support Category will be added to menu.json")
		return data, nil
	}

	data.insertCategories(supportCategory)
	return data, nil
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
