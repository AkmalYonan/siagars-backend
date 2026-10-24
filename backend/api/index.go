// Package handler is the Vercel serverless entrypoint; all routes are rewritten here (see vercel.json).
package handler

import (
	"net/http"

	"siagars/backend/serverless"
)

// Handler forwards every request to the shared API router.
func Handler(w http.ResponseWriter, r *http.Request) {
	serverless.ServeHTTP(w, r)
}
