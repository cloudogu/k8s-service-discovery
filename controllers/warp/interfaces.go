package warp

import (
	"context"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"k8s.io/client-go/tools/record"
)

// Reader is used to fetch warp categories with a configuration
type Reader interface {
	Read(context.Context, *config.Configuration) (types.Categories, error)
}

type eventRecorder interface {
	record.EventRecorder
}

type watchConfigurationContext interface {
	registry.WatchConfigurationContext
}

// DoguConverter is used to Read dogus from the registry and convert them to objects fitting in the warp menu
type DoguConverter interface {
	CreateEntryWithCategoryFromDogu(dogu *core.Dogu, tag string) (types.EntryWithCategory, error)
}

// ExternalConverter is used to Read external links from the registry and convert them to objects fitting in the warp menu
type ExternalConverter interface {
	ReadAndUnmarshalExternal(registry types.WatchConfigurationContext, key string) (types.EntryWithCategory, error)
}

type DoguVersionRegistry interface {
	WatchAllCurrent(context.Context) (dogu.CurrentVersionsWatch, error)
	GetCurrentOfAll(context.Context) ([]dogu.DoguVersion, error)
}

type LocalDoguRepo interface {
	GetAll(context.Context, []dogu.DoguVersion) (map[dogu.DoguVersion]*core.Dogu, error)
}
