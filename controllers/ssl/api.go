package ssl

import (
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/cesapp-lib/ssl"
	"github.com/gin-gonic/gin"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
)

const endpointPostGenerateSSL = "/api/v1/ssl"

var logger = ctrl.Log.WithName("k8s-service-discovery")

type globalConfig interface {
	registry.ConfigurationContext
}

type cesSelfSignedSSLGenerator interface {
	// GenerateSelfSignedCert generates a self-signed certificate for the ces and returns the certificate chain and the
	// private key as string.
	GenerateSelfSignedCert(fqdn string, domain string, certExpireDays int, country string,
		province string, locality string, altDNSNames []string) (string, string, error)
}

type cesSSLWriter interface {
	// WriteCertificate writes the type, cert and key to the global config
	WriteCertificate(certType string, cert string, key string) error
}

type ginRouter interface {
	gin.IRoutes
}

// SetupAPI setups the REST API for ssl generation
func SetupAPI(router ginRouter, globalConfig globalConfig) {
	logger.Info("Register endpoint [%s][%s]", http.MethodPost, endpointPostGenerateSSL)

	router.POST(endpointPostGenerateSSL, func(ctx *gin.Context) {
		handleSSLRequest(ctx, globalConfig, ssl.NewSSLGenerator(), NewSSLWriter(globalConfig))
	})
}

func handleSSLRequest(ctx *gin.Context, globalConfig globalConfig, sslGenerator cesSelfSignedSSLGenerator, sslWriter cesSSLWriter) {
	validDays := ctx.Query("days")
	i, err := strconv.ParseInt(validDays, 10, 0)
	if err != nil {
		handleError(ctx, http.StatusBadRequest, err, "Expire days can't convert to integer")
		return
	}

	fqdn, err := globalConfig.Get("fqdn")
	if err != nil {
		handleError(ctx, http.StatusInternalServerError, err, "Failed to get FQDN from global config")
		return
	}

	domain, err := globalConfig.Get("domain")
	if err != nil {
		handleError(ctx, http.StatusInternalServerError, err, "Failed to get DOMAIN from global config")
		return
	}

	cert, key, err := sslGenerator.GenerateSelfSignedCert(fqdn, domain, int(i), ssl.Country, ssl.Province, ssl.Locality, []string{})
	if err != nil {
		handleError(ctx, http.StatusInternalServerError, err, "Failed to generate self-signed certificate and key")
		return
	}

	err = sslWriter.WriteCertificate("selfsigned", cert, key)
	if err != nil {
		handleError(ctx, http.StatusInternalServerError, err, "Failed to write certificate to global config")
		return
	}

	ctx.Status(http.StatusOK)
}

func handleError(ginCtx *gin.Context, httpCode int, err error, causingAction string) {
	logger.Error(err, "ssl api error")
	ginCtx.String(httpCode, "HTTP %d: An error occurred during this action: %s",
		httpCode, causingAction)
	ginCtx.Writer.WriteHeaderNow()
	ginCtx.Abort()
	_ = ginCtx.Error(err)
}
