window.ui = SwaggerUIBundle({
  url: "/docs/openapi.json",
  dom_id: "#swagger-ui",
  deepLinking: true,
  // Sends the HttpOnly session cookie on "Try it out" requests to the same origin.
  withCredentials: true,
  presets: [SwaggerUIBundle.presets.apis],
});
