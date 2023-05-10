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

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{}).Return(nil)

		// when
		handleSSLRequest(ginCtx, creatorMock)

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
		handleSSLRequest(ginCtx, nil)

		// then
		assert.Equal(t, http.StatusBadRequest, ginCtx.Writer.Status())
		assert.Contains(t, recorder.Body.String(), "Expire days can't convert to integer")
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
