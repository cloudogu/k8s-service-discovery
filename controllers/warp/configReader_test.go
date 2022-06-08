package warp

import (
	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfigReader_readSupport(t *testing.T) {
	supportSources := []config.SupportSource{{Identifier: "aboutCloudoguToken", External: false, Href: "/local/href"}, {Identifier: "myCloudogu", External: true, Href: "https://ecosystem.cloudogu.com/"}, {Identifier: "docsCloudoguComUrl", External: true, Href: "https://docs.cloudogu.com/"}}
	reader := &ConfigReader{
		configuration: &config.Configuration{Support: []config.SupportSource{}},
		registry:      nil,
	}
	t.Run("success with one disabled entry", func(t *testing.T) {
		// given
		expectedCategories := types.Categories{{Title: "Support", Entries: []types.Entry{
			{Title: "aboutCloudoguToken", Target: types.TARGET_SELF, Href: "/local/href"},
			{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"}}}}

		// when
		actual := reader.readSupport(supportSources, []string{"docsCloudoguComUrl"})

		// then
		assert.Equal(t, expectedCategories, actual, "readSupport did not return the correct Category of two entries")
	})

	t.Run("success with empty filter", func(t *testing.T) {
		// given
		expectedCategories := types.Categories{
			{Title: "Support", Entries: []types.Entry{
				{Title: "aboutCloudoguToken", Target: types.TARGET_SELF, Href: "/local/href"},
				{Title: "myCloudogu", Target: types.TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
				{Title: "docsCloudoguComUrl", Target: types.TARGET_EXTERNAL, Href: "https://docs.cloudogu.com/"}}}}

		// when
		actual := reader.readSupport(supportSources, []string{})

		// then
		assert.Equal(t, expectedCategories, actual)
	})

	t.Run("success with complete filter", func(t *testing.T) {
		// given
		expectedCategories := types.Categories{}

		// when
		actual := reader.readSupport(supportSources, []string{"myCloudogu", "aboutCloudoguToken", "docsCloudoguComUrl"})

		// then
		assert.Equal(t, 0, expectedCategories.Len())
		assert.Equal(t, expectedCategories, actual, "readSupport did not return the correct Category of three entries")
	})
}

func TestConfigReader_getDisabledSupportIdentifiers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}

		// when
		identifiers, err := reader.getDisabledSupportIdentifiers()

		// then
		assert.Empty(t, err)
		assert.Equal(t, []string{"lorem", "ipsum"}, identifiers)
		mock.AssertExpectationsForObjects(t, mockRegistry)
	})

	t.Run("failed to get disabled support entries", func(t *testing.T) {
		// given
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("", assert.AnError)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}

		// when
		_, err := reader.getDisabledSupportIdentifiers()

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to Read configuration entry /config/_global/disabled_warpmenu_support_entries from etcd")
		mock.AssertExpectationsForObjects(t, mockRegistry)
	})

	t.Run("failed to unmarshal disabled support entries", func(t *testing.T) {
		// given
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("{\"lorem\": \"ipsum\"}", nil)
		reader := &ConfigReader{
			configuration: &config.Configuration{Support: []config.SupportSource{}},
			registry:      mockRegistry,
		}

		// when
		_, err := reader.getDisabledSupportIdentifiers()

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal etcd key")
		mock.AssertExpectationsForObjects(t, mockRegistry)
	})
}

func TestConfigReader_readFromConfig(t *testing.T) {
	mockRegistry := &cesmocks.WatchConfigurationContext{}
	mockRegistry.On("GetChildrenPaths", "/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
	mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
	testSources := []config.Source{{Path: "/path/to/etcd/key", Type: "externals", Tag: "tag"}, {Path: "/path", Type: "disabled_support_entries"}}
	testSupportSoureces := []config.SupportSource{{Identifier: "supportSrc", External: true, Href: "path/to/external"}}
	mockDoguConverter := &mocks.DoguConverter{}

	t.Run("success with one external and support link", func(t *testing.T) {
		// given
		cloudoguEntryWithCategory := getEntryWithCategory("Cloudogu", "www.cloudogu.com", "Cloudogu", "External", types.TARGET_EXTERNAL)
		mockExternalConverter := &mocks.ExternalConverter{}
		mockExternalConverter.On("ReadAndUnmarshalExternal", mockRegistry, mock.Anything).Return(cloudoguEntryWithCategory, nil)
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
		mockDoguConverter := &mocks.DoguConverter{}
		mockExternalConverter := &mocks.ExternalConverter{}
		doguSource := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", mock.Anything).Return([]string{}, nil)
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)
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
		mockExternalConverter := &mocks.ExternalConverter{}
		mockExternalConverter.On("ReadAndUnmarshalExternal", mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
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
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", "/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", assert.AnError)
		mockExternalConverter := &mocks.ExternalConverter{}
		mockExternalConverter.On("ReadAndUnmarshalExternal", mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
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
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", "/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
		mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[]", nil)
		mockExternalConverter := &mocks.ExternalConverter{}
		mockExternalConverter.On("ReadAndUnmarshalExternal", mockRegistry, mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
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
}

func TestConfigReader_dogusReader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", "/dogu").Return([]string{"/dogu/redmine", "/dogu/jenkins"}, nil)
		redmineEntryWithCategory := getEntryWithCategory("Redmine", "/redmine", "Redmine", "Development Apps", types.TARGET_SELF)
		jenkinsEntryWithCategory := getEntryWithCategory("Jenkins", "/jenkins", "Jenkins", "Development Apps", types.TARGET_SELF)
		mockDoguConverter := &mocks.DoguConverter{}
		mockDoguConverter.On("ReadAndUnmarshalDogu", mockRegistry, "/dogu/redmine", "warp").Return(redmineEntryWithCategory, nil)
		mockDoguConverter.On("ReadAndUnmarshalDogu", mockRegistry, "/dogu/jenkins", "warp").Return(jenkinsEntryWithCategory, nil)
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
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", "/dogu").Return([]string{}, assert.AnError)
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
		mockRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("GetChildrenPaths", "/path").Return([]string{}, assert.AnError)
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
