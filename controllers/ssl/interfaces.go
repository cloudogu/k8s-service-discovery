package ssl

import (
	"context"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type GlobalConfigRepository interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
	Update(ctx context.Context, globalConfig libconfig.GlobalConfig) (libconfig.GlobalConfig, error)
}

type SecretClient interface {
	corev1.SecretInterface
}
