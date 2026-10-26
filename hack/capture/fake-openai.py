# Minimal OpenAI-compatible server for captures: GET /v1/models lists a fixed base model, so
# an Open WebUI connection pointed at it exposes that model. Run with the Open WebUI image's
# Python; it needs nothing beyond the standard library.
import http.server
import json

MODELS = {"object": "list", "data": [{"id": "capture-base", "object": "model", "created": 1700000000, "owned_by": "capture"}]}


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        found = self.path.rstrip("/").endswith("/models")
        body = json.dumps(MODELS if found else {"error": "not found"}).encode()
        self.send_response(200 if found else 404)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(body)


http.server.HTTPServer(("", 8000), Handler).serve_forever()
