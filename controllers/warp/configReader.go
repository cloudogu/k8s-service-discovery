package warp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/pkg/errors"
)

// ConfigReader reads the configuration for the warp menu from etcd
type ConfigReader struct {
	configuration     *config.Configuration
	registry          watchConfigurationContext
	doguConverter     DoguConverter
	externalConverter ExternalConverter
}

// DoguConverter is used to Read dogus from the registry and convert them to objects fitting in the warp menu
type DoguConverter interface {
	ReadAndUnmarshalDogu(registry types.WatchConfigurationContext, key string, tag string) (types.EntryWithCategory, error)
}

// ExternalConverter is used to Read external links from the registry and convert them to objects fitting in the warp menu
type ExternalConverter interface {
	ReadAndUnmarshalExternal(registry types.WatchConfigurationContext, key string) (types.EntryWithCategory, error)
}

const blockWarpSupportCategoryConfigurationKey = "/config/_global/block_warpmenu_support_category"
const disabledWarpSupportEntriesConfigurationKey = "/config/_global/disabled_warpmenu_support_entries"
const allowedWarpSupportEntriesConfigurationKey = "/config/_global/allowed_warpmenu_support_entries"

// Read reads sources specified in a configuration and build warp menu categories for them.
func (reader *ConfigReader) Read(configuration *config.Configuration) (types.Categories, error) {
	var data types.Categories

	for _, source := range configuration.Sources {
		// Disabled support entries refresh every time
		if source.Type == "support_entry_config" {
			continue
		}

		categories, err := reader.readSource(source)
		if err != nil {
			ctrl.Log.Info(fmt.Sprintf("Error during Read: %s", err.Error()))
		}
		data.InsertCategories(categories)
	}

	ctrl.Log.Info("Read SupportEntries")

	isSupportCategoryBlocked, err := reader.readBool(blockWarpSupportCategoryConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf("Warning, could not read etcd Key: %v. Err: %v", blockWarpSupportCategoryConfigurationKey, err))
	}

	disabledSupportEntries, err := reader.readStrings(disabledWarpSupportEntriesConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf("Warning, could not read etcd Key: %v. Err: %v", disabledWarpSupportEntriesConfigurationKey, err))
	}

	allowedSupportEntries, err := reader.readStrings(allowedWarpSupportEntriesConfigurationKey)
	if err != nil {
		ctrl.Log.Info(fmt.Sprintf("Warning, could not read etcd Key: %v. Err: %v", allowedWarpSupportEntriesConfigurationKey, err))
	}

	supportCategory := reader.readSupport(configuration.Support, isSupportCategoryBlocked, disabledSupportEntries, allowedSupportEntries)
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
	ctrl.Log.Info(fmt.Sprintf("Read externals from %s for warp menu", source.Path))
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to Read root entry %s from etcd: %w", source.Path, err)
	}
	var externals []types.EntryWithCategory
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
	ctrl.Log.Info(fmt.Sprintf("Read dogus from %s for warp menu", source.Path))
	resp, err := reader.registry.GetChildrenPaths(source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to Read root entry %s from etcd: %w", source.Path, err)
	}
	var dogus []types.EntryWithCategory
	for _, path := range resp {
		dogu, err := reader.doguConverter.ReadAndUnmarshalDogu(reader.registry, path, source.Tag)
		if err == nil && dogu.Entry.Title != "" {
			dogus = append(dogus, dogu)
		}
	}

	return reader.createCategories(dogus), nil
}

func (reader *ConfigReader) readStrings(registryKey string) ([]string, error) {
	entry, err := reader.registry.Get(registryKey)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read configuration entry %s from etcd: %w", registryKey, err)
	}

	var strings []string
	err = json.Unmarshal([]byte(entry), &strings)
	if err != nil {
		return []string{}, fmt.Errorf("failed to unmarshal etcd key to string slice: %w", err)
	}

	return strings, nil
}

func (reader *ConfigReader) readBool(registryKey string) (bool, error) {
	entry, err := reader.registry.Get(registryKey)
	if err != nil {
		return false, fmt.Errorf("failed to read configuration entry %s from etcd: %w", registryKey, err)
	}

	boolValue, err := strconv.ParseBool(entry)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal etcd key to bool: %w", err)
	}

	return boolValue, nil
}

func (reader *ConfigReader) readSupport(supportSources []config.SupportSource, blocked bool, disabledEntries []string, allowedEntries []string) types.Categories {
	var supportEntries []types.EntryWithCategory

	for _, supportSource := range supportSources {
		if (blocked && StringInSlice(supportSource.Identifier, allowedEntries)) || (!blocked && !StringInSlice(supportSource.Identifier, disabledEntries)) {
			// support category is blocked, but this entry is explicitly allowed OR support category is NOT blocked and this entry is NOT explicitly disabled

			entry := types.Entry{Title: supportSource.Identifier, Href: supportSource.Href, Target: types.TARGET_SELF}
			if supportSource.External {
				entry.Target = types.TARGET_EXTERNAL
			}

			supportEntries = append(supportEntries, types.EntryWithCategory{Entry: entry, Category: "Support"})
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
