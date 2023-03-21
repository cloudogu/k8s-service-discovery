package ssl

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func Test_handleSSLRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=1"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate("selfsigned", "mycert", "mykey").Return(nil)

		// when
		handleSSLRequest(ginCtx, globalConfigMock, sslGeneratorMock, sslWriterMock)

		// then
		assert.Equal(t, http.StatusOK, ginCtx.Writer.Status())
	})

	t.Run("should return an error of days param can not be parsed", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=sdf"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request

		// when
		handleSSLRequest(ginCtx, nil, nil, nil)

		// then
		assert.Equal(t, http.StatusBadRequest, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Expire days can't convert to integer")
	})

	t.Run("should return an error if fqdn can not be queried", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=1"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request

		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("", assert.AnError)

		// when
		handleSSLRequest(ginCtx, globalConfigMock, nil, nil)

		// then
		assert.Equal(t, http.StatusInternalServerError, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Failed to get FQDN from global config")
	})

	t.Run("should return an error if domain can not be queried", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=1"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request

		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("", assert.AnError)

		// when
		handleSSLRequest(ginCtx, globalConfigMock, nil, nil)

		// then
		assert.Equal(t, http.StatusInternalServerError, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Failed to get DOMAIN from global config")
	})

	t.Run("should return an error if certificate can not be generated", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=1"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request

		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", assert.AnError)

		// when
		handleSSLRequest(ginCtx, globalConfigMock, sslGeneratorMock, nil)

		// then
		assert.Equal(t, http.StatusInternalServerError, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Failed to generate self-signed certificate and key")
	})

	t.Run("should return an error if certificate can not be written", func(t *testing.T) {
		// given
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		reqUrl := &url.URL{RawQuery: "http://localhost:9090/api/v1/setup&days=1"}
		request := &http.Request{URL: reqUrl}
		ginCtx.Request = request

		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate("selfsigned", "mycert", "mykey").Return(assert.AnError)

		// when
		handleSSLRequest(ginCtx, globalConfigMock, sslGeneratorMock, sslWriterMock)

		// then
		assert.Equal(t, http.StatusInternalServerError, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Failed to write certificate to global config")
	})
}

func TestSetupAPI(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		routerMock := newMockGinRouter(t)
		routerMock.EXPECT().POST("/api/v1/ssl", mock.Anything).Return(nil)

		// when
		SetupAPI(routerMock, nil)
	})
}
