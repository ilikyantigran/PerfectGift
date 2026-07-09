// Package docs serves the gateway's public OpenAPI (Swagger) contract and a Swagger
// UI page. Unlike the domain services (whose spec is generated from proto), the
// gateway OWNS its OpenAPI document by hand — it is the client contract. The
// canonical file is ./openapi.yaml at the module root; `make docs` copies it here so
// it can be embedded at build time and the binary stays self-contained.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiYAML []byte

// OpenAPISpec returns the raw embedded OpenAPI document (used e.g. in tests).
func OpenAPISpec() []byte { return openapiYAML }

const swaggerUI = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>PerfectGift API Gateway · Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({ url: "./openapi.yaml", dom_id: "#swagger-ui" });
    };
  </script>
</body>
</html>`

// Handler serves the spec at /openapi.yaml and the Swagger UI at /.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openapiYAML)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUI))
	})
	return mux
}
