// Package docs embeds the OpenAPI contract and the API reference page served at /docs.
package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed reference.html
var ReferencePage []byte
