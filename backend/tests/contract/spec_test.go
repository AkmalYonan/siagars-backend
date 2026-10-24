package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// spec is docs/openapi.yaml (JSON syntax) plus a validator for the OpenAPI 3.0 subset the contract uses.
type spec struct{ root map[string]any }

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func decodeJSON(b []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(target)
}
func loadSpec(t testing.TB) spec {
	t.Helper()
	b, e := os.ReadFile("../../docs/openapi.yaml")
	if e != nil {
		t.Fatal(e)
	}
	var root map[string]any
	if e = decodeJSON(b, &root); e != nil {
		t.Fatal("openapi.yaml harus JSON valid:", e)
	}
	return spec{root}
}
func loadFixtures(t testing.TB) map[string]map[string]any {
	t.Helper()
	b, e := os.ReadFile("../../docs/fixtures.json")
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]map[string]any
	if e = decodeJSON(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func (s spec) ref(name string) map[string]any {
	return obj(obj(obj(s.root["components"])["schemas"])[name])
}
func (s spec) resolve(schema map[string]any) map[string]any {
	for {
		r, ok := schema["$ref"].(string)
		if !ok {
			return schema
		}
		schema = s.ref(strings.TrimPrefix(r, "#/components/schemas/"))
	}
}
func (s spec) paths() map[string]map[string]any {
	out := map[string]map[string]any{}
	for p, v := range obj(s.root["paths"]) {
		out[p] = obj(v)
	}
	return out
}

// responseSchema returns the JSON schema documented for method+path+status, or ok=false if the status is undocumented.
func (s spec) responseSchema(method, path string, status int) (map[string]any, bool) {
	op := obj(s.paths()[path][strings.ToLower(method)])
	r := obj(obj(op["responses"])[strconv.Itoa(status)])
	if r == nil {
		return nil, false
	}
	return obj(obj(obj(r["content"])["application/json"])["schema"]), true
}

// queryParam returns the schema of a documented query parameter of GET /hospitals.
func (s spec) queryParam(name string) map[string]any {
	op := obj(s.paths()["/hospitals"]["get"])
	params, _ := op["parameters"].([]any)
	for _, p := range params {
		if m := obj(p); m["name"] == name {
			return obj(m["schema"])
		}
	}
	return nil
}
func (s spec) validate(schema map[string]any, val any) []string {
	var errs []string
	s.check(schema, val, "$", &errs)
	return errs
}
func (s spec) check(schema map[string]any, val any, at string, errs *[]string) {
	bad := func(format string, a ...any) { *errs = append(*errs, at+": "+fmt.Sprintf(format, a...)) }
	if schema == nil {
		return
	}
	if val == nil {
		if schema["nullable"] != true {
			bad("null tidak diizinkan")
		}
		return
	}
	schema = s.resolve(schema)
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			found = found || fmt.Sprint(e) == fmt.Sprint(val)
		}
		if !found {
			bad("nilai %v di luar enum %v", val, enum)
		}
	}
	switch schema["type"] {
	case "object":
		m, ok := val.(map[string]any)
		if !ok {
			bad("harus object, dapat %T", val)
			return
		}
		props := obj(schema["properties"])
		if req, ok := schema["required"].([]any); ok {
			for _, k := range req {
				if _, has := m[k.(string)]; !has {
					bad("field wajib %q hilang", k)
				}
			}
		}
		for k, v := range m {
			p, known := props[k]
			switch {
			case known:
				s.check(obj(p), v, at+"."+k, errs)
			case schema["additionalProperties"] == false:
				bad("field %q tidak ada di kontrak", k)
			case obj(schema["additionalProperties"]) != nil:
				s.check(obj(schema["additionalProperties"]), v, at+"."+k, errs)
			}
		}
	case "array":
		a, ok := val.([]any)
		if !ok {
			bad("harus array, dapat %T", val)
			return
		}
		for i, v := range a {
			s.check(obj(schema["items"]), v, fmt.Sprintf("%s[%d]", at, i), errs)
		}
	case "string":
		str, ok := val.(string)
		if !ok {
			bad("harus string, dapat %T", val)
			return
		}
		n := len([]rune(str))
		if f, ok := schema["minLength"].(json.Number); ok && float(f) > float64(n) {
			bad("lebih pendek dari minLength")
		}
		if f, ok := schema["maxLength"].(json.Number); ok && float(f) < float64(n) {
			bad("lebih panjang dari maxLength")
		}
		switch schema["format"] {
		case "uuid":
			if !uuidPattern.MatchString(str) {
				bad("bukan UUID huruf kecil: %q", str)
			}
		case "date-time":
			if _, e := time.Parse(time.RFC3339, str); e != nil || !strings.HasSuffix(str, "Z") {
				bad("bukan RFC3339 UTC: %q", str)
			}
		case "uri":
			if u, e := url.Parse(str); e != nil || u.Scheme == "" {
				bad("bukan URI: %q", str)
			}
		case "email":
			if !strings.Contains(str, "@") {
				bad("bukan email: %q", str)
			}
		}
	case "integer", "number":
		n, ok := val.(json.Number)
		if !ok {
			bad("harus %v, dapat %T", schema["type"], val)
			return
		}
		if schema["type"] == "integer" {
			if _, e := strconv.ParseInt(n.String(), 10, 64); e != nil {
				bad("bukan integer: %s", n)
				return
			}
		}
		if m, ok := schema["minimum"].(json.Number); ok && float(n) < float(m) {
			bad("%s < minimum %s", n, m)
		}
		if m, ok := schema["maximum"].(json.Number); ok && float(n) > float(m) {
			bad("%s > maximum %s", n, m)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			bad("harus boolean, dapat %T", val)
		}
	}
}
func float(n json.Number) float64 { f, _ := n.Float64(); return f }
