package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestAPIBodyLimit(t *testing.T) {
	e := echo.New()
	e.Use(apiBodyLimit())
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	e.POST(debugUploadPath, ok, middleware.BodyLimit("20M"))
	e.POST("/v1/reviewctl/upload/:projectKey/", ok)

	post := func(path string, size int) int {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(make([]byte, size))))
		return rec.Code
	}
	const mb = 1 << 20

	assert.Equal(t, http.StatusOK, post("/v1/upload/debug/key/", 5*mb), "debug bundles get the 20MB route cap")
	assert.Equal(t, http.StatusRequestEntityTooLarge, post("/v1/upload/debug/key/", 21*mb))
	assert.Equal(t, http.StatusOK, post("/v1/reviewctl/upload/key/", mb))
	assert.Equal(t, http.StatusRequestEntityTooLarge, post("/v1/reviewctl/upload/key/", 3*mb), "other routes keep the 2MB cap")
}
