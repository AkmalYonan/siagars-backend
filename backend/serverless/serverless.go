// Package serverless exposes the API to the Vercel entrypoint in api/, which Vercel compiles outside this
// module's import path and therefore cannot import internal packages directly.
package serverless

import (
	"net/http"

	"siagars/backend/internal/app"
)

// ServeHTTP serves one request through the shared, lazily initialised API router.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.ServeHTTP(w, r)
}
