package warp

import (
	"context"
	_ "embed"
	"github.com/cloudogu/cesapp-lib/core"
	registryconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

var testCtx = context.Background()

//go:embed testdata/redmine.json
var redmineBytes []byte

//go:embed testdata/jenkins.json
var jenkinsBytes []byte

func TestConfigReader_readSupport(t *testing.T) {
	supportSources := []config.SupportSource{{Identifier: "aboutCloudoguToken", External: false, Href: "/local/href"}, {Identifier: "myCloudogu", External: true, Href: "https://ecosystem.cloudogu.com/"}, {Identifier: "docsCloudoguComUrl", External: true, Href: "https://docs.cloudogu.com/"}}
	reader := &ConfigReader{
		configuration: &config.Configuration{Support: []config.SupportSource{}},
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
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"disabled_warpmenu_support_entries": "[\"lorem\",\"ipsum\"]",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		identifiers, err := reader.readStrings(testCtx, "disabled_warpmenu_support_entries")
		require.NoError(t, err)
		assert.Equal(t, []string{"lorem", "ipsum"}, identifiers)
	})

	t.Run("should fail getting global config", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(registryconfig.GlobalConfig{}, assert.AnError)
		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		_, err := reader.readStrings(testCtx, "disabled_warpmenu_support_entries")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get global config")
	})

	t.Run("should fail unmarshalling", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"disabled_warpmenu_support_entries": "not a string array]",
			}),
		}

		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		identifiers, err := reader.readStrings(testCtx, "disabled_warpmenu_support_entries")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to unmarshal global config key to string slice")
		assert.Equal(t, []string{}, identifiers)
	})
}

func TestConfigReader_readBool(t *testing.T) {
	t.Run("should successfully read true bool", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"myBool": "true",
			}),
		}

		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		boolValue, err := reader.readBool(testCtx, "myBool")
		require.NoError(t, err)
		assert.True(t, boolValue)
	})

	t.Run("should successfully read false bool", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"myBool": "false",
			}),
		}

		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		boolValue, err := reader.readBool(testCtx, "myBool")
		require.NoError(t, err)
		assert.False(t, boolValue)
	})

	t.Run("should fail getting global config", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(registryconfig.GlobalConfig{}, assert.AnError)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		_, err := reader.readBool(testCtx, "myBool")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get global config")
	})

	t.Run("should fail unmarshalling", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)

		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"myBool": "not a pool",
			}),
		}

		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		reader := &ConfigReader{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		boolValue, err := reader.readBool(testCtx, "myBool")
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to unmarshal value \"not a pool\" to bool")
		assert.False(t, boolValue)
	})
}
func TestConfigReader_readFromConfig(t *testing.T) {

	testSources := []config.Source{{Path: "/path/to/external/link", Type: "externals", Tag: "tag"}, {Path: "/path", Type: "support_entry_config"}}
	testSupportSources := []config.SupportSource{{Identifier: "supportSrc", External: true, Href: "path/to/external"}}

	t.Run("success with one external and support link", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"/path/to/external/link/Cloudogu":               "[\"lorem\", \"ipsum\"]",
				globalBlockWarpSupportCategoryConfigurationKey:  "false",
				globalAllowedWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockDoguConverter := NewMockDoguConverter(t)

		cloudoguEntryWithCategory := getEntryWithCategory("Cloudogu", "www.cloudogu.com", "Cloudogu", "External", types.TARGET_EXTERNAL)
		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mock.Anything).Return(cloudoguEntryWithCategory, nil)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			globalConfigRepo:  mockGlobalConfigRepo,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(testCtx, &config.Configuration{Sources: testSources, Support: testSupportSources})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 2, len(actual))
	})

	t.Run("success with one dogu and support link", func(t *testing.T) {
		// given

		mockDoguConverter := NewMockDoguConverter(t)
		mockDoguConverter.EXPECT().CreateEntryWithCategoryFromDogu(readRedmineDogu(t), "warp").Return(types.EntryWithCategory{Entry: types.Entry{DisplayName: "Redmine", Title: "Redmine"}, Category: "Development Apps"}, nil)
		mockExternalConverter := NewMockExternalConverter(t)
		doguSource := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}

		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"disabled_warpmenu_support_entries":             "[\"lorem\", \"ipsum\"]",
				globalBlockWarpSupportCategoryConfigurationKey:  "false",
				globalAllowedWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		versionRegistryMock := NewMockDoguVersionRegistry(t)
		redmineVersion := parseVersion(t, "5.1.3-1")
		redmineDoguVersion := dogu.DoguVersion{Name: "redmine", Version: *redmineVersion}
		currentDoguVersions := []dogu.DoguVersion{redmineDoguVersion}
		versionRegistryMock.EXPECT().GetCurrentOfAll(testCtx).Return(currentDoguVersions, nil)
		doguSpecRepoMock := NewMockLocalDoguRepo(t)
		doguSpecRepoMock.EXPECT().GetAll(testCtx, currentDoguVersions).Return(map[dogu.DoguVersion]*core.Dogu{redmineDoguVersion: readRedmineDogu(t)}, nil)

		reader := &ConfigReader{
			configuration:       &config.Configuration{Support: []config.SupportSource{}},
			globalConfigRepo:    mockGlobalConfigRepo,
			doguConverter:       mockDoguConverter,
			externalConverter:   mockExternalConverter,
			doguVersionRegistry: versionRegistryMock,
			localDoguRepo:       doguSpecRepoMock,
		}

		// when
		actual, err := reader.Read(testCtx, &config.Configuration{Sources: []config.Source{doguSource}, Support: testSupportSources})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 2, len(actual))
	})

	t.Run("error during external Read should not result in an error", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"/path/to/external/link/Cloudogu":               "[\"lorem\", \"ipsum\"]",
				"disabled_warpmenu_support_entries":             "[\"lorem\", \"ipsum\"]",
				globalAllowedWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
				globalBlockWarpSupportCategoryConfigurationKey:  "false",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockDoguConverter := NewMockDoguConverter(t)

		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			globalConfigRepo:  mockGlobalConfigRepo,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(testCtx, &config.Configuration{Sources: testSources, Support: testSupportSources})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
	})

	t.Run("empty support category should not result in an error", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"/path/to/external/link/Cloudogu":               "[\"lorem\", \"ipsum\"]",
				"disabled_warpmenu_support_entries":             "[]",
				globalAllowedWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
				globalBlockWarpSupportCategoryConfigurationKey:  "false",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockExternalConverter := NewMockExternalConverter(t)
		mockExternalConverter.EXPECT().ReadAndUnmarshalExternal(mock.Anything).Return(types.EntryWithCategory{}, assert.AnError)
		mockDoguConverter := NewMockDoguConverter(t)
		reader := &ConfigReader{
			configuration:     &config.Configuration{Support: []config.SupportSource{}},
			globalConfigRepo:  mockGlobalConfigRepo,
			doguConverter:     mockDoguConverter,
			externalConverter: mockExternalConverter,
		}

		// when
		actual, err := reader.Read(testCtx, &config.Configuration{Sources: testSources, Support: testSupportSources})

		// then
		assert.Empty(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, 1, len(actual))
	})

	t.Run("skip wrong source type", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"/path/to/external/link/Cloudogu":               "[\"lorem\", \"ipsum\"]",
				globalAllowedWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
				globalBlockWarpSupportCategoryConfigurationKey:  "false",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		testSources := []config.Source{{Path: "/path/to/etcd/key", Type: "fjkhsdfjh", Tag: "tag"}}
		reader := &ConfigReader{
			configuration:    &config.Configuration{Support: []config.SupportSource{}},
			globalConfigRepo: mockGlobalConfigRepo,
		}
		// when
		_, err := reader.Read(testCtx, &config.Configuration{Sources: testSources, Support: testSupportSources})

		// then
		require.NoError(t, err)
	})

	t.Run("should read categories from config", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"externals/ext1": "external",
				globalAllowedWarpSupportEntriesConfigurationKey:  "[\"lorem\", \"ipsum\"]",
				globalDisabledWarpSupportEntriesConfigurationKey: "[\"lorem\", \"ipsum\"]",
			}),
		}
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockConverter := NewMockExternalConverter(t)
		mockConverter.EXPECT().ReadAndUnmarshalExternal("external").Return(types.EntryWithCategory{
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
			globalConfigRepo:  mockGlobalConfigRepo,
			externalConverter: mockConverter,
		}

		testSources := []config.Source{{Path: "externals", Type: "externals", Tag: "tag"}}
		testSupportSoureces := []config.SupportSource{{Identifier: "supportSrc", External: true, Href: "https://support.source"}}

		actual, err := reader.Read(testCtx, &config.Configuration{Sources: testSources, Support: testSupportSoureces})
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
}

func TestConfigReader_dogusReader(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		redmineEntryWithCategory := getEntryWithCategory("Redmine", "/redmine", "Redmine", "Development Apps", types.TARGET_SELF)
		jenkinsEntryWithCategory := getEntryWithCategory("Jenkins", "/jenkins", "Jenkins", "Development Apps", types.TARGET_SELF)
		mockDoguConverter := NewMockDoguConverter(t)
		mockDoguConverter.EXPECT().CreateEntryWithCategoryFromDogu(readRedmineDogu(t), "warp").Return(redmineEntryWithCategory, nil)
		mockDoguConverter.EXPECT().CreateEntryWithCategoryFromDogu(readJenkinsDogu(t), "warp").Return(jenkinsEntryWithCategory, nil)
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		redmineVersion := parseVersion(t, "5.1.3-1")
		jenkinsVersion := parseVersion(t, "2.452.2-1")
		redmineDoguVersion := dogu.DoguVersion{Name: "redmine", Version: *redmineVersion}
		jenkinsDoguVersion := dogu.DoguVersion{Name: "jenkins", Version: *jenkinsVersion}
		currentDoguVersions := []dogu.DoguVersion{redmineDoguVersion, jenkinsDoguVersion}
		versionRegistryMock.EXPECT().GetCurrentOfAll(testCtx).Return(currentDoguVersions, nil)
		doguSpecRepoMock := NewMockLocalDoguRepo(t)
		doguSpecRepoMock.EXPECT().GetAll(testCtx, currentDoguVersions).Return(map[dogu.DoguVersion]*core.Dogu{redmineDoguVersion: readRedmineDogu(t), jenkinsDoguVersion: readJenkinsDogu(t)}, nil)

		reader := &ConfigReader{
			configuration:       &config.Configuration{Support: []config.SupportSource{}},
			doguConverter:       mockDoguConverter,
			doguVersionRegistry: versionRegistryMock,
			localDoguRepo:       doguSpecRepoMock,
		}

		// when
		categories, err := reader.dogusReader(testCtx, source)

		// then
		require.NoError(t, err)
		assert.Equal(t, 1, categories.Len())
		assert.Equal(t, 2, len(categories[0].Entries))
	})

	t.Run("failed to get all current versions", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		versionRegistryMock.EXPECT().GetCurrentOfAll(testCtx).Return(nil, assert.AnError)
		reader := &ConfigReader{
			doguVersionRegistry: versionRegistryMock,
			configuration:       &config.Configuration{Support: []config.SupportSource{}},
		}

		// when
		_, err := reader.dogusReader(testCtx, source)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get all current dogu versions")
	})

	t.Run("failed to get dogus of currents", func(t *testing.T) {
		// given
		source := config.Source{
			Path: "/dogu",
			Type: "dogus",
			Tag:  "warp",
		}
		redmineVersion := parseVersion(t, "5.1.3-1")
		redmineDoguVersion := dogu.DoguVersion{Name: "redmine", Version: *redmineVersion}
		currentDoguVersions := []dogu.DoguVersion{redmineDoguVersion}
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		versionRegistryMock.EXPECT().GetCurrentOfAll(testCtx).Return(currentDoguVersions, nil)
		doguSpecMock := NewMockLocalDoguRepo(t)
		doguSpecMock.EXPECT().GetAll(testCtx, currentDoguVersions).Return(nil, assert.AnError)
		reader := &ConfigReader{
			doguVersionRegistry: versionRegistryMock,
			localDoguRepo:       doguSpecMock,
			configuration:       &config.Configuration{Support: []config.SupportSource{}},
		}

		// when
		_, err := reader.dogusReader(testCtx, source)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get all dogu specs with current versions")
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
