package litellm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceLiteLLMModels() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceLiteLLMModelsRead,

		Schema: map[string]*schema.Schema{
			"model_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Distinct list of all model names registered on the gateway",
			},
			"models": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Full list of model deployments registered on the gateway",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"litellm_model_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Internal LiteLLM deployment ID",
						},
						"model_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Public model name exposed by the gateway",
						},
						"custom_llm_provider": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "LLM provider (e.g. azure, vertex_ai, bedrock)",
						},
						"base_model": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Base model used for pricing lookups",
						},
						"mode": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Model mode (chat, embedding, completion, etc.)",
						},
						"tier": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Model tier (free, paid, etc.)",
						},
					},
				},
			},
		},
	}
}

func dataSourceLiteLLMModelsRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)

	resp, err := MakeRequest(client, "GET", endpointModelInfo, nil)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(bodyBytes))
	}

	var listResp ModelInfoListResponse
	if err := json.Unmarshal(bodyBytes, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Build deduplicated model_names list and full models list.
	seen := make(map[string]bool)
	var modelNames []string
	var models []map[string]interface{}

	for _, model := range listResp.Data {
		if !seen[model.ModelName] {
			seen[model.ModelName] = true
			modelNames = append(modelNames, model.ModelName)
		}
		models = append(models, map[string]interface{}{
			"litellm_model_id":    model.ModelInfo.ID,
			"model_name":          model.ModelName,
			"custom_llm_provider": model.LiteLLMParams.CustomLLMProvider,
			"base_model":          model.ModelInfo.BaseModel,
			"mode":                model.ModelInfo.Mode,
			"tier":                model.ModelInfo.Tier,
		})
	}

	d.SetId("litellm_models")
	d.Set("model_names", modelNames)
	d.Set("models", models)

	return nil
}
