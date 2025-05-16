package warp

import (
	"context"
	"github.com/cloudogu/cesapp-lib/core"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/warp/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reader is used to fetch warp categories with a configuration
type Reader interface {
	Read(context.Context, *config.Configuration) (types.Categories, error)
}

type eventRecorder interface {
	record.EventRecorder
}

// DoguConverter is used to Read dogus from the registry and convert them to objects fitting in the warp menu
type DoguConverter interface {
	CreateEntryWithCategoryFromDogu(dogu *core.Dogu, tag string) (types.EntryWithCategory, error)
}

// ExternalConverter is used to Read external links from the registry and convert them to objects fitting in the warp menu
type ExternalConverter interface {
	ReadAndUnmarshalExternal(link string) (types.EntryWithCategory, error)
}

type DoguVersionRegistry interface {
	WatchAllCurrent(context.Context) (<-chan dogu.CurrentVersionsWatchResult, error)
	GetCurrentOfAll(context.Context) ([]dogu.DoguVersion, error)
}

type LocalDoguRepo interface {
	GetAll(context.Context, []dogu.DoguVersion) (map[dogu.DoguVersion]*core.Dogu, error)
}

type GlobalConfigRepository interface {
	Watch(context.Context, ...libconfig.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)
	Get(context.Context) (libconfig.GlobalConfig, error)
}

// used for mocks

//nolint:unused
//goland:noinspection GoUnusedType
type k8sClient interface {
	client.Client
}
