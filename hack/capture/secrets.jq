# What counts as a secret in captured exchanges; included by normalize.jq and capture.sh.

# Values that look like credentials wherever they appear: Open WebUI API keys and JWTs.
def secret_re: "sk-[0-9a-f]{32}|eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+";

# Keys whose string values (or arrays of strings) are secrets, e.g. token, api_key, OPENAI_API_KEYS.
def secret_key_re: "(^|_)(tokens?|secrets?|passwords?|api_keys?)$";

# secret_values lists every string held under a secret key, anywhere in the input.
def secret_values:
  .. | objects | to_entries[] | select(.key | test(secret_key_re; "i"))
  | .value | if type == "array" then .[] else . end | select(type == "string");
