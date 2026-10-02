package litellm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestKeyMCPCreatePayload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]interface{}
		want   interface{}
	}{
		{"omitted", map[string]interface{}{}, nil},
		{"null", map[string]interface{}{"mcp_servers": nil}, nil},
		{"empty", map[string]interface{}{"mcp_servers": []interface{}{}}, map[string]interface{}{"mcp_servers": []interface{}{}}},
		{"granted", map[string]interface{}{"mcp_servers": []interface{}{"server-1"}}, map[string]interface{}{"mcp_servers": []interface{}{"server-1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var payload map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/key/generate":
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"key": "sk-fixture", "token_id": "hash-1"})
				case "/key/info":
					json.NewEncoder(w).Encode(map[string]interface{}{"info": map[string]interface{}{"object_permission": payload["object_permission"]}})
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			res := resourceKey()
			diff, err := res.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(tc.config), nil)
			if err != nil {
				t.Fatal(err)
			}
			// Legacy SDK test helpers do not populate RawConfig. Supply the
			// same null/empty/nonempty distinction Terraform sends over RPC.
			raw := cty.NullVal(cty.Set(cty.String))
			if values, ok := tc.config["mcp_servers"].([]interface{}); ok {
				raw = cty.SetValEmpty(cty.String)
				if len(values) > 0 {
					items := make([]cty.Value, len(values))
					for i, value := range values {
						items[i] = cty.StringVal(value.(string))
					}
					raw = cty.SetVal(items)
				}
			}
			diff.RawConfig = cty.ObjectVal(map[string]cty.Value{"mcp_servers": raw})
			d, err := schema.InternalMap(res.Schema).Data(nil, diff)
			if err != nil {
				t.Fatal(err)
			}
			if diags := resourceKeyCreate(context.Background(), d, NewClient(srv.URL, "fixture", false)); diags.HasError() {
				t.Fatal(diags)
			}
			if !reflect.DeepEqual(payload["object_permission"], tc.want) {
				t.Fatalf("object_permission = %#v, want %#v", payload["object_permission"], tc.want)
			}
		})
	}
}

func TestKeyMCPReadClearsStaleState(t *testing.T) {
	for _, permission := range []interface{}{nil, map[string]interface{}{}, map[string]interface{}{"mcp_servers": []interface{}{}}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]interface{}{"info": map[string]interface{}{"object_permission": permission}})
		}))
		d := newKeyResourceData(t, map[string]interface{}{"mcp_servers": []interface{}{"stale"}})
		d.SetId("hash-1")
		diags := resourceKeyRead(context.Background(), d, NewClient(srv.URL, "fixture", false))
		srv.Close()
		if diags.HasError() {
			t.Fatal(diags)
		}
		if d.Get("mcp_servers").(*schema.Set).Len() != 0 {
			t.Fatal("read retained stale MCP grants")
		}
	}
}

func TestKeyMCPUpdatesArePartialAndOmissionIsUnmanaged(t *testing.T) {
	permission := map[string]interface{}{"mcp_servers": []interface{}{"server-1", "server-2"}, "vector_stores": []interface{}{"store-1"}}
	row := map[string]interface{}{"key_alias": "service", "max_budget": float64(125), "object_permission": permission}
	var writes []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/key/update":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			writes = append(writes, body)
			for k, v := range body {
				if k == "object_permission" {
					for pk, pv := range v.(map[string]interface{}) {
						permission[pk] = pv
					}
				} else if k != "key" {
					row[k] = v
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "hash-1"})
		case "/key/info":
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "hash-1", "info": row})
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client := NewClient(srv.URL, "fixture", false)
	r := resourceKey()
	d := newKeyResourceData(t, map[string]interface{}{"key_alias": "service", "max_budget": 125, "mcp_servers": []interface{}{"server-1", "server-2"}})
	d.SetId("hash-1")
	state := d.State()
	apply := func(config map[string]interface{}) {
		t.Helper()
		diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), client)
		if err != nil {
			t.Fatal(err)
		}
		if diff == nil || diff.Empty() {
			return
		}
		next, diagnostics := r.Apply(context.Background(), state, diff, client)
		if diagnostics.HasError() {
			t.Fatal(diagnostics)
		}
		state = next
	}
	// The budget changes externally after Terraform's last refresh. A partial
	// MCP update must not write the stale value of 125 back over 250.
	row["max_budget"] = float64(250)
	apply(map[string]interface{}{"key_alias": "service", "max_budget": 125, "mcp_servers": []interface{}{"server-1"}})
	want := map[string]interface{}{"key": "hash-1", "object_permission": map[string]interface{}{"mcp_servers": []interface{}{"server-1"}}}
	if len(writes) != 1 || !reflect.DeepEqual(writes[0], want) {
		t.Fatalf("MCP-only payload: %#v", writes)
	}
	if row["max_budget"] != float64(250) {
		t.Fatal("overwrote external budget")
	}
	if !reflect.DeepEqual(permission["vector_stores"], []interface{}{"store-1"}) {
		t.Fatal("overwrote unrelated object permissions")
	}

	// Omitted means stop managing, not revoke. Another key edit must not carry
	// object_permission merely because the last read returned it.
	apply(map[string]interface{}{"key_alias": "renamed", "max_budget": 250})
	if len(writes) != 2 || !reflect.DeepEqual(writes[1], map[string]interface{}{"key": "hash-1", "key_alias": "renamed"}) {
		t.Fatalf("unmanaged permissions were written: %#v", writes)
	}
	apply(map[string]interface{}{"key_alias": "renamed", "max_budget": 250, "mcp_servers": []interface{}{}})
	want["object_permission"] = map[string]interface{}{"mcp_servers": []interface{}{}}
	if len(writes) != 3 || !reflect.DeepEqual(writes[2], want) {
		t.Fatalf("empty list not sent explicitly: %#v", writes)
	}
	apply(map[string]interface{}{"key_alias": "renamed", "max_budget": 250, "mcp_servers": []interface{}{}})
	if len(writes) != 3 {
		t.Fatal("unchanged apply issued another write")
	}

	// A combined edit sends explicitly changed budgets as well as MCP grants.
	apply(map[string]interface{}{"key_alias": "renamed", "max_budget": 300, "mcp_servers": []interface{}{"server-2"}})
	want["object_permission"] = map[string]interface{}{"mcp_servers": []interface{}{"server-2"}}
	want["max_budget"] = float64(300)
	if len(writes) != 4 || !reflect.DeepEqual(writes[3], want) {
		t.Fatalf("combined update lost a field: %#v", writes)
	}

	// Refresh observes remote removal, and the subsequent apply repairs it.
	permission["mcp_servers"] = []interface{}{}
	refreshed, diags := r.RefreshWithoutUpgrade(context.Background(), state, client)
	if diags.HasError() {
		t.Fatal(diags)
	}
	state = refreshed
	apply(map[string]interface{}{"key_alias": "renamed", "max_budget": 300, "mcp_servers": []interface{}{"server-2"}})
	delete(want, "max_budget")
	if len(writes) != 5 || !reflect.DeepEqual(writes[4], want) {
		t.Fatalf("drift not repaired: %#v", writes)
	}
}
