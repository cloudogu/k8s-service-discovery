package ssl

import (
	"context"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
)

type GlobalConfigRepository interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
	Update(ctx context.Context, globalConfig libconfig.GlobalConfig) (libconfig.GlobalConfig, error)
}
