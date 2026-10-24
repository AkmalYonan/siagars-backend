// Package docs embeds the OpenAPI contract and the Swagger UI page served at /docs.
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed swagger.html
var SwaggerPage []byte

//go:embed swagger-init.js
var SwaggerInit []byte
