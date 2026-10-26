# Minimal OpenAI-compatible server for captures: GET /v1/models lists a fixed base model, so
# an Open WebUI connection pointed at it exposes that model, and POST /v1/embeddings returns a
# deterministic 8-number vector (values in [0, 1]) per input, so files can be ingested without a
# real embedding engine.
# Run with the Open WebUI image's Python; it needs nothing beyond the standard library.
import hashlib
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

    def do_POST(self):
        if not self.path.rstrip("/").endswith("/embeddings"):
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length") or 0)
        request = json.loads(self.rfile.read(length) or b"{}")
        inputs = request.get("input") or []
        inputs = [inputs] if isinstance(inputs, str) else inputs
        data = [{"object": "embedding", "index": i, "embedding": vector(text)} for i, text in enumerate(inputs)]
        body = json.dumps({"object": "list", "data": data, "model": request.get("model")}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(body)


def vector(text):
    """Eight numbers in [0, 1] derived from the text's hash."""
    digest = hashlib.sha256(str(text).encode()).digest()
    return [byte / 255 for byte in digest[:8]]


http.server.HTTPServer(("", 8000), Handler).serve_forever()
