package ssl

import (
	"github.com/cloudogu/cesapp-lib/ssl"
	"github.com/gin-gonic/gin"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
)

const endpointPostGenerateSSL = "/api/v1/ssl"

var logger = ctrl.Log.WithName("k8s-service-discovery")

type selfSignedCertificateCreator interface {
	CreateAndSafeCertificate(certExpireDays int, country string,
		province string, locality string, altDNSNames []string) error
}

type ginRouter interface {
	gin.IRoutes
}

// SetupAPI setups the REST API for ssl generation
func SetupAPI(router ginRouter, globalConfig globalConfig) {
	logger.Info("Register endpoint [%s][%s]", http.MethodPost, endpointPostGenerateSSL)

	router.POST(endpointPostGenerateSSL, func(ctx *gin.Context) {
		handleSSLRequest(ctx, NewCreator(globalConfig))
	})
}

func handleSSLRequest(ctx *gin.Context, certificateCreator selfSignedCertificateCreator) {
	validDays := ctx.Query("days")
	i, err := strconv.ParseInt(validDays, 10, 0)
	if err != nil {
		handleError(ctx, http.StatusBadRequest, err, "Expire days can't convert to integer")
		return
	}

	err = certificateCreator.CreateAndSafeCertificate(int(i), ssl.Country, ssl.Province, ssl.Locality, []string{})
	if err != nil {
		handleError(ctx, http.StatusInternalServerError, err, "Failed to create and write certificate to global config")
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
