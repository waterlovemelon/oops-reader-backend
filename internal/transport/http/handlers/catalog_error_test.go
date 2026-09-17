package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
)

// The error mapping is the contract clients branch on: a width the caller
// cannot fix is a 400, a missing image variant and a missing resource are 404s,
// and anything else must not reach the client as a success.
//
// The body is only asserted where it is part of that contract; an unexpected
// failure keeps its status guarantee without pinning the internal message.
func TestWriteCatalogErrorMapsFailuresToStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{
			name:   "invalid image width",
			err:    catalog.ErrInvalidImageWidth,
			status: http.StatusBadRequest,
			body:   `{"error":"width_px must be a positive integer"}`,
		},
		{
			name:   "missing image variant",
			err:    catalog.ErrImageVariantNotFound,
			status: http.StatusNotFound,
			body:   `{"error":"image variant not found"}`,
		},
		{
			name:   "missing resource",
			err:    catalog.ErrNotFound,
			status: http.StatusNotFound,
			body:   `{"error":"not found"}`,
		},
		{
			name:   "unexpected failure",
			err:    errors.New("database is down"),
			status: http.StatusInternalServerError,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)

			writeCatalogError(context, testCase.err)

			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.status)
			}
			if testCase.body != "" && recorder.Body.String() != testCase.body {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), testCase.body)
			}
		})
	}
}
