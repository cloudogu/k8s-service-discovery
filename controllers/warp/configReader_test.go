package warp

import (
	"bytes"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
)

func TestConfigReader_readSupport(t *testing.T) {
	supportSources := []config.SupportSource{{Identifier: "aboutCloudoguToken", External: false, Href: "/local/href"}, {Identifier: "myCloudogu", External: true, Href: "https://ecosystem.cloudogu.com/"}, {Identifier: "docsCloudoguComUrl", External: true, Href: "https://docs.cloudogu.com/"}}
	reader := &ConfigReader{
		configuration: &config.Configuration{Support: []config.SupportSource{}},
		registry:      nil,
	}

	t.Run("should successfully read support entries without filters", func(t *testing.T) {
		actual := reader.readSupport(supportSources, false, []string{}, []string{})

		expectedCategories := types.Categories{
			{Title: "Support", Entries: []types.Entry{
				{Title: "aboutCloudoguToken", Target: types.TARGET_SELF, Href: "/local/href"},
				{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
				{Title: "docsCloudoguComUrl", Target: types.TARGET_EXTERNAL, Href: "https://docs.cloudogu.com/"},
			}}}
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("should block all entries", func(t *testing.T) {
		actual := reader.readSupport(supportSources, true, []string{}, []string{})

		expectedCategories := types.Categories{}
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("should add allowed entries when blocked", func(t *testing.T) {
		actual := reader.readSupport(supportSources, true, []string{}, []string{"myCloudogu"})

		expectedCategories := types.Categories{
			{Title: "Support", Entries: []types.Entry{
				{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
			}}}
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("should remove disabled entries when not blocked", func(t *testing.T) {
		actual := reader.readSupport(supportSources, false, []string{"aboutCloudoguToken", "docsCloudoguComUrl"}, []string{})

		expectedCategories := types.Categories{
			{Title: "Support", Entries: []types.Entry{
				{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
			}}}
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("should remove disabled entries when not blocked", func(t *testing.T) {
		actual := reader.readSupport(supportSources, false, []string{"aboutCloudoguToken", "docsCloudoguComUrl"}, []string{})

		expectedCategories := types.Categories{
			{Title: "Support", Entries: []types.Entry{
				{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
			}}}
		assert.Equal(t, expectedCategories, actual)
	})
}

func TestConfigReader_readStrings(t *testing.T) {
	t.Run("should successfully read strings", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		identifiers, err := reader.readStrings("/config/_global/disabled_warpmenu_support_entries")
		require.NoError(t, err)
		assert.Equal(t, []string{"lorem", "ipsum"}, identifiers)
	})

	t.Run("should fail reading from registry", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("", assert.AnError)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		identifiers, err := reader.readStrings("/config/_global/disabled_warpmenu_support_entries")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to read configuration entry /config/_global/disabled_warpmenu_support_entries from etcd")
		assert.Equal(t, []string{}, identifiers)
	})

	t.Run("should fail unmarshalling", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("not-a-string-array 123", nil)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		identifiers, err := reader.readStrings("/config/_global/disabled_warpmenu_support_entries")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to unmarshal etcd key to string slice:")
		assert.Equal(t, []string{}, identifiers)
	})
}

func TestConfigReader_readBool(t *testing.T) {
	t.Run("should successfully read true bool", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/myBool").Return("true", nil)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		boolValue, err := reader.readBool("/config/_global/myBool")
		require.NoError(t, err)
		assert.True(t, boolValue)
	})

	t.Run("should successfully read false bool", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/myBool").Return("false", nil)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		boolValue, err := reader.readBool("/config/_global/myBool")
		require.NoError(t, err)
		assert.False(t, boolValue)
	})

	t.Run("should fail reading from registry", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/myBool").Return("", assert.AnError)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		boolValue, err := reader.readBool("/config/_global/myBool")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to read configuration entry /config/_global/myBool from etcd")
		assert.False(t, boolValue)
	})

	t.Run("should fail unmarshalling", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/myBool").Return("not a bool", nil)

		reader := &ConfigReader{
			registry: mockRegistry,
		}

		boolValue, err := reader.readBool("/config/_global/myBool")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to unmarshal etcd key to bool:")
		assert.False(t, boolValue)
	})
}

func TestConfigReader_readFromConfig(t *testing.T) {

	testSources := []config.Source{{Path: "/path/to/etcd/key", Type: "externals", Tag: "tag"}, {Path: "/path", Type: "support_entry_config"}}
	testSupportSoureces := []config.SupportSource{{Identifier: "supportSrc", External: true, Href: "path/to/external"}}

	t.Run("success with one external and support link", func(t *testing.T) {
		// given
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		mockDoguConverter := NewMockDoguConverter(t)

		cloudoguEntryWithCategory := getEntryWithCategory("Cloudogu", "www.cloudogu.com", "Cloudogu", "External", types.TARGET_EXTERNAL)
		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mockRegistry, mock.Anything).Return(cloudoguEntryWithCategory, nil)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 2, len(actual))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter, mockExternalConverter)
	})

	t.Run("success with one dogu and support link", func(t *testing.T) {
		// given

		mockDoguConverter := NewMockDoguConverter(t)
		mockExternalConverter := NewMockExternalConverter(t)
		doguSource := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths(mock.Anything).Return([]string{}, nil)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(&config.Configuration{Sources: []config.Source{doguSource}, Support: testSupportSoureces})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter, mockExternalConverter)
	})

	t.Run("error during external Read should not result in an error", func(t *testing.T) {
		// given
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		mockDoguConverter := NewMockDoguConverter(t)

		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter, mockExternalConverter)
	})

	t.Run("error during support Read should not result in an error", func(t *testing.T) {
		// given
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", assert.AnError)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
		mockDoguConverter := NewMockDoguConverter(t)

		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter, mockExternalConverter)
	})

	t.Run("empty support category should not result in an error", func(t *testing.T) {
		// given
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[]", nil)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
		mockDoguConverter := NewMockDoguConverter(t)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter, mockExternalConverter)
	})

	t.Run("skip wrong source type", func(t *testing.T) {
		// given
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get("/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)

		testSources := []config.Source{{Path: "/path/to/etcd/key", Type: "fjkhsdfjh", Tag: "tag"}}
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}
		// when
		_, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})

		// then
		require.NoError(t, err)
	})

	t.Run("should read categories from config", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).
			Return("false", nil)
		mockRegistry.EXPECT().Get(disabledWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).
			Return("[\"lorem\", \"ipsum\"]", nil)
		mockRegistry.EXPECT().GetChildrenPaths("/config/externals").
			Return([]string{"/config/externals/ext1"}, nil)

		mockConverter := NewMockExternalConverter(t)
		mockConverter.EXPECT().ReadAndUnmarshalExternal(mockRegistry, "/config/externals/ext1").Return(types.EntryWithCategory{
			Entry: types.Entry{
				DisplayName: "ext1",
				Href:        "https://my.url/ext1",
				Title:       "ext1 Description",
				Target:      types.TARGET_EXTERNAL,
			},
			Category: "Documentation",
		}, nil)

		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			externalConverter: mockConverter,
		}

		testSources := []config.Source{{Path: "/config/externals", Type: "externals", Tag: "tag"}}
		testSupportSoureces := []config.SupportSource{{Identifier: "supportSrc", External: true, Href: "https://support.source"}}

		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSoureces})
		require.NoError(t, err)

		expectedCategories := types.Categories{
			{Title: "Documentation", Entries: []types.Entry{
				{DisplayName: "ext1", Title: "ext1 Description", Target: types.TARGET_EXTERNAL, Href: "https://my.url/ext1"},
			}},
			{Title: "Support", Entries: []types.Entry{
				{Title: "supportSrc", Target: types.TARGET_EXTERNAL, Href: "https://support.source"},
			}},
		}
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("should read categories from config", func(t *testing.T) {
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().Get(blockWarpSupportCategoryConfigurationKey).Return("", assert.AnError)
		mockRegistry.EXPECT().Get(disabledWarpSupportEntriesConfigurationKey).Return("", assert.AnError)
		mockRegistry.EXPECT().Get(allowedWarpSupportEntriesConfigurationKey).Return("", assert.AnError)
		mockRegistry.EXPECT().GetChildrenPaths("/config/externals").Return(nil, assert.AnError)

		mockConverter := NewMockExternalConverter(t)

		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			registry:          mockRegistry,
			externalConverter: mockConverter,
		}

		testSources := []config.Source{{Path: "/config/externals", Type: "externals", Tag: "tag"}}
		testSupportSources := []config.SupportSource{}

		// capture log
		var buf bytes.Buffer
		existingLog := ctrl.Log
		ctrl.Log = funcr.New(func(prefix, args string) {
			buf.WriteString(fmt.Sprintf("[%s] %s\n", prefix, args))
		}, funcr.Options{})
		defer func() {
			ctrl.Log = existingLog
		}()

		actual, err := reader.Read(&config.Configuration{Sources: testSources, Support: testSupportSources})
		require.NoError(t, err)

		assert.Nil(t, actual)

		// assert log
		assert.Contains(t, buf.String(), "Error during Read: failed to Read root entry /config/externals from etcd")
		assert.Contains(t, buf.String(), "Warning, could not read etcd Key: /config/_global/block_warpmenu_support_category.")
		assert.Contains(t, buf.String(), "Warning, could not read etcd Key: /config/_global/disabled_warpmenu_support_entries.")
		assert.Contains(t, buf.String(), "Warning, could not read etcd Key: /config/_global/allowed_warpmenu_support_entries.")
	})
}

func TestConfigReader_dogusReader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/dogu").Return([]string{"/dogu/redmine", "/dogu/jenkins"}, nil)
		redmineEntryWithCategory := getEntryWithCategory("Redmine", "/redmine", "Redmine", "Development Apps", types.TARGET_SELF)
		jenkinsEntryWithCategory := getEntryWithCategory("Jenkins", "/jenkins", "Jenkins", "Development Apps", types.TARGET_SELF)
		mockDoguConverter := NewMockDoguConverter(t)
		mockDoguConverter.EXPECT().ReadAndUnmarshalDogu(mockRegistry, "/dogu/redmine", "warp").Return(redmineEntryWithCategory, nil)
		mockDoguConverter.EXPECT().ReadAndUnmarshalDogu(mockRegistry, "/dogu/jenkins", "warp").Return(jenkinsEntryWithCategory, nil)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
			doguConverter: mockDoguConverter,
		}

		// when
		categories, err := reader.dogusReader(source)

		// then
		require.NoError(t, err)
		assert.Equal(t, 1, categories.Len())
		assert.Equal(t, 2, len(categories[0].Entries))
		mock.AssertExpectationsForObjects(t, mockRegistry, mockDoguConverter)
	})

	t.Run("failed to get children of /dogu path", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/dogu").Return([]string{}, assert.AnError)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}

		// when
		_, err := reader.dogusReader(source)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to Read root entry /dogu from etcd")
		mock.AssertExpectationsForObjects(t, mockRegistry)
	})
}

func getEntryWithCategory(displayName string, href string, title string, category string, target types.Target) types.EntryWithCategory {
	return types.EntryWithCategory{Entry: types.Entry{
		DisplayName: displayName,
		Href:        href,
		Title:       title,
		Target:      target,
	}, Category: category}
}

func TestConfigReader_externalsReader(t *testing.T) {
	t.Run("fail to get children paths", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/path",
			Type: "externals",
		}
		mockRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().GetChildrenPaths("/path").Return([]string{}, assert.AnError)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}

		// when
		_, err := reader.externalsReader(source)

		// then
		require.Error(t, err)
		mock.AssertExpectationsForObjects(t, mockRegistry)
	})
}
