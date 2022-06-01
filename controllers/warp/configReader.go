package warp

import (
	"encoding/json"
	"github.com/cloudogu/cesapp-lib/registry"
	"log"

	"sort"

	"github.com/pkg/errors"
)

// ConfigReader reads the configuration for the warp menu from etcd
type ConfigReader struct {
	configuration *Configuration
	registry      registry.WatchConfigurationContext
}

type DisabledSupportEntries struct {
	name []string
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
func (reader *ConfigReader) dogusReader(source Source) (Categories, error) {
	log.Printf("read dogus from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read root entry %s from etcd", source.Path)
	}
	dogus := []EntryWithCategory{}
	for _, child := range resp {
		dogu, err := readAndUnmarshalDogu(reader.registry, child, source.Tag)
		if err != nil {
			log.Printf("failed to read and unmarshal dogu: %v", err)
		} else if dogu.Entry.Title != "" {
			dogus = append(dogus, dogu)
		}
	}

	return reader.createCategories(dogus), nil
}

func (reader *ConfigReader) externalsReader(source Source) (Categories, error) {
	log.Printf("read externals from %s for warp menu", source.Path)
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read root entry %s from etcd", source.Path)
	}
	externals := []EntryWithCategory{}
	for _, child := range resp {
		external, err := readAndUnmarshalExternal(reader.registry, child)
		if err != nil {
			log.Printf("failed to read and unmarshal external: %v", err)
		} else {
			externals = append(externals, external)
		}
	}
	return reader.createCategories(externals), nil
}

func (reader *ConfigReader) readSource(source Source) (Categories, error) {
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
		return []string{}, errors.Wrapf(err, "failed to read configuration entry %s from etcd", disableWarpSupportEntriesConfigurationKey)
	}

	var disabledEntries []string
	err = json.Unmarshal([]byte(disabledSupportEntries), &disabledEntries)
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to unmarshal etcd key")
	}

	return disabledEntries, nil
}

func (reader *ConfigReader) readSupport(supportSources []SupportSource, disabledSupportEntries []string) (Categories, error) {
	var supportEntries []EntryWithCategory

	for _, supportSource := range supportSources {
		// supportSource -> EntryWithCategory
		if !StringInSlice(supportSource.Identifier, disabledSupportEntries) {
			entry := Entry{}
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

func (reader *ConfigReader) readFromConfig(configuration *Configuration) (Categories, error) {
	var data Categories

	for _, source := range configuration.Sources {
		categories, err := reader.readSource(source)
		if err != nil {
			log.Println("Error during read:", err)
		}
		data.insertCategories(categories)
	}

	log.Println("read SupportEntries")
	disabledSupportEntries, err := reader.getDisabledSupportIdentifiers()
	if err != nil {
		log.Printf("Warning, could not read etcd Key: %v. Err: %v", disableWarpSupportEntriesConfigurationKey, err)
	}
	// add support Category
	supportCategory, err := reader.readSupport(configuration.Support, disabledSupportEntries)
	if err != nil {
		log.Println("Error during support read:", err)
	}
	if supportCategory.Len() == 0 {
		log.Printf("support Category is empty, no support Category will be added to menu.json")
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