package ssl

import (
	"context"
	"fmt"
)

const (
	serverCertificateKeyID = "certificate/server.key"
)

type certificateKeyRemover struct {
	globalConfigRepo GlobalConfigRepository
}

func NewCertificateKeyRemover(globalConfigRepo GlobalConfigRepository) *certificateKeyRemover {
	return &certificateKeyRemover{
		globalConfigRepo: globalConfigRepo,
	}
}

func (ckr *certificateKeyRemover) RemoveCertificateKeyOutOfGlobalConfig(ctx context.Context) error {
	globalConfig, err := ckr.globalConfigRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config object: %w", err)
	}

	// delete private key since it is a security risk
	globalConfig.Config = globalConfig.Delete(serverCertificateKeyID)

	_, err = ckr.globalConfigRepo.Update(ctx, globalConfig)
	if err != nil {
		return fmt.Errorf("failed to write global config object: %w", err)
	}

	return nil
}

func (ckr *certificateKeyRemover) Start(ctx context.Context) error {
	return ckr.RemoveCertificateKeyOutOfGlobalConfig(ctx)
}
