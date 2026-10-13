package litellm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TF_ACC=1 TF_ACC_TERRAFORM_PATH=/path/to/terraform go test ./litellm -run TestAccKeyMCPPermissions
// Exercises real Terraform/plugin RPC against a local HTTP fixture, not a live proxy.
func TestAccKeyMCPPermissions(t *testing.T) {
	var mu sync.Mutex
	var row map[string]interface{}
	var updates []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/key/generate":
			if err := json.NewDecoder(r.Body).Decode(&row); err != nil {
				t.Error(err)
			}
			if _, ok := row["object_permission"]; !ok {
				row["object_permission"] = map[string]interface{}{"mcp_servers": []interface{}{"default-server"}}
			}
			row["object_permission"].(map[string]interface{})["vector_stores"] = []interface{}{"store-1"}
			row["max_budget"] = float64(250)
			row["soft_budget"] = float64(200)
			row["budget_duration"] = "monthly"
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "sk-fixture", "token_id": "hash-1"})
		case "/key/info":
			if row == nil {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{}`)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "hash-1", "info": row})
		case "/key/update":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			updates = append(updates, body)
			for name, value := range body {
				if name == "object_permission" {
					for k, v := range value.(map[string]interface{}) {
						row[name].(map[string]interface{})[k] = v
					}
				} else if name != "key" {
					row[name] = value
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"key": "hash-1"})
		case "/key/delete":
			row = nil
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	config := func(mcp string) string {
		return fmt.Sprintf(`provider "litellm" {
  api_base = %q
  api_key = "fixture"
}
resource "litellm_key" "test" {
  key_alias = "test"
  %s
  lifecycle { ignore_changes = [budget_duration] }
}`, srv.URL, mcp)
	}
	check := func(want []interface{}, count int) resource.TestCheckFunc {
		return func(_ *terraform.State) error {
			mu.Lock()
			defer mu.Unlock()
			permission := row["object_permission"].(map[string]interface{})
			if !reflect.DeepEqual(permission["mcp_servers"], want) {
				return fmt.Errorf("MCP grants = %#v, want %#v", permission["mcp_servers"], want)
			}
			if len(updates) != count {
				return fmt.Errorf("updates = %#v, want %d", updates, count)
			}
			if row["max_budget"] != float64(250) || row["soft_budget"] != float64(200) || row["budget_duration"] != "monthly" {
				return fmt.Errorf("budget changed: %#v", row)
			}
			if !reflect.DeepEqual(permission["vector_stores"], []interface{}{"store-1"}) {
				return fmt.Errorf("unrelated permissions changed: %#v", permission)
			}
			for _, body := range updates {
				if len(body) != 2 || body["key"] != "hash-1" {
					return fmt.Errorf("non-MCP fields sent: %#v", body)
				}
				fields, ok := body["object_permission"].(map[string]interface{})
				if !ok || len(fields) != 1 || fields["mcp_servers"] == nil {
					return fmt.Errorf("invalid partial permission: %#v", body)
				}
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		ProviderFactories: map[string]func() (*schema.Provider, error){"litellm": func() (*schema.Provider, error) { return Provider(), nil }},
		Steps: []resource.TestStep{
			{Config: config(""), Check: check([]interface{}{"default-server"}, 0)},
			{Config: config(`mcp_servers = ["server-1", "server-2"]`), Check: check([]interface{}{"server-1", "server-2"}, 1)},
			{Config: config(`mcp_servers = ["server-2", "server-1"]`), PlanOnly: true},
			{Config: config(`mcp_servers = ["server-1"]`), Check: check([]interface{}{"server-1"}, 2)},
			{Config: config(`mcp_servers = []`), Check: check([]interface{}{}, 3)},
			{Config: config(`mcp_servers = ["server-1"]`), Check: check([]interface{}{"server-1"}, 4)},
			{Config: config(""), Check: check([]interface{}{"server-1"}, 4)},
			{Config: config(`mcp_servers = null`), Check: check([]interface{}{"server-1"}, 4)},
			{Config: config(`mcp_servers = ["server-1"]`), PreConfig: func() {
				mu.Lock()
				defer mu.Unlock()
				row["object_permission"].(map[string]interface{})["mcp_servers"] = []interface{}{"unexpected-server"}
			}, Check: check([]interface{}{"server-1"}, 5)},
			{ResourceName: "litellm_key.test", ImportState: true, ImportStateVerify: true},
			{Config: config(`mcp_servers = []`), Destroy: true},
			{Config: config(`mcp_servers = []`), Check: check([]interface{}{}, 5)},
			{Config: config(`mcp_servers = []`), Destroy: true},
			{Config: config(`mcp_servers = ["server-1"]`), Check: check([]interface{}{"server-1"}, 5)},
		},
	})
}
