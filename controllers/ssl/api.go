package ssl

import (
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/cesapp-lib/ssl"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"

	"github.com/gin-gonic/gin"
)

const endpointPostGenerateSSL = "/api/v1/ssl"

var logger = ctrl.Log.WithName("k8s-service-discovery")

// SetupAPI setups the REST API for ssl generation
func SetupAPI(router gin.IRoutes, etcdRegistry registry.Registry) {
	logger.Info("Register endpoint [%s][%s]", http.MethodPost, endpointPostGenerateSSL)

	router.POST(endpointPostGenerateSSL, func(ctx *gin.Context) {
		validDays := ctx.Query("days")
		i, err := strconv.ParseInt(validDays, 10, 0)
		if err != nil {
			handleError(ctx, http.StatusBadRequest, err, "Expire days can't convert to integer")
			return
		}

		config := etcdRegistry.GlobalConfig()
		fqdn, err := config.Get("fqdn")
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, err, "Failed to get FQDN from global config")
			return
		}

		domain, err := config.Get("domain")
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, err, "Failed to get DOMAIN from global config")
			return
		}

		sslGenerator := ssl.NewSSLGenerator()
		cert, key, err := sslGenerator.GenerateSelfSignedCert(fqdn, domain, int(i), ssl.Country, ssl.Province, ssl.Locality, nil)
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, err, "Failed to generate self-signed certificate and key")
			return
		}

		sslWriter := NewSSLWriter(config)
		err = sslWriter.WriteCertificate("selfsigned", cert, key)
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, err, "Failed to write certificate to global config")
			return
		}

		ctx.Status(http.StatusOK)
	})
}

func handleError(ginCtx *gin.Context, httpCode int, err error, causingAction string) {
	logger.Error(err, "ssl api error")
	ginCtx.String(httpCode, "HTTP %d: An error occurred during this action: %s",
		httpCode, causingAction)
	ginCtx.Writer.WriteHeaderNow()
	ginCtx.Abort()
	_ = ginCtx.Error(err)
}
