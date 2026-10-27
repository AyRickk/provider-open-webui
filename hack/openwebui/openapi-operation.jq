# Prints one operation of an OpenAPI snapshot in a comparable form: its parameters, request body
# and 200 response, with every $ref inlined (recursion cut at 8 levels). The annotation keys
# summary, description, title and operationId are dropped, since they change without changing the
# contract; keys of a `properties` map are field names, so a field called `description` is kept.
# Arguments: --arg method <get|post|delete|…> --arg path <OpenAPI path>.

def annotation: IN("summary", "description", "title", "operationId");

def resolve($root; $depth):
  if $depth > 8 then "<recursion>"
  elif type == "object" then
    if has("$ref") then
      (.["$ref"] | sub("^#/components/schemas/"; "")) as $name
      | $root.components.schemas[$name] | resolve($root; $depth + 1)
    else
      to_entries
      | map(if .key == "properties" and (.value | type) == "object" then .value |= map_values(resolve($root; $depth))
            elif .key | annotation then empty
            else .value |= resolve($root; $depth) end)
      | from_entries
    end
  elif type == "array" then map(resolve($root; $depth))
  else . end;

. as $root
| .paths[$path][$method]
| if . == null then null
  else {
    parameters: ([.parameters[]? | {name, in, required: (.required // false), schema}] | sort_by(.in, .name)),
    request: .requestBody.content["application/json"].schema,
    response: .responses["200"].content["application/json"].schema
  } | resolve($root; 0)
  end
