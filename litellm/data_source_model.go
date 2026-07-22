package litellm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceLiteLLMModel() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceLiteLLMModelRead,

		Schema: map[string]*schema.Schema{
			"litellm_model_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Internal LiteLLM model ID (litellm_model_id) of the deployment to retrieve",
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
	}
}

func dataSourceLiteLLMModelRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	modelID := d.Get("litellm_model_id").(string)

	resp, err := MakeRequest(client, "GET", fmt.Sprintf("%s?litellm_model_id=%s", endpointModelInfo, modelID), nil)
	if err != nil {
		return fmt.Errorf("failed to read model: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("model with litellm_model_id %q not found", modelID)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(bodyBytes))
	}

	var listResp ModelInfoListResponse
	if err := json.Unmarshal(bodyBytes, &listResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(listResp.Data) == 0 {
		return fmt.Errorf("model with litellm_model_id %q not found", modelID)
	}

	model := listResp.Data[0]

	d.SetId(modelID)
	d.Set("model_name", model.ModelName)
	d.Set("custom_llm_provider", model.LiteLLMParams.CustomLLMProvider)
	d.Set("base_model", model.ModelInfo.BaseModel)
	d.Set("mode", model.ModelInfo.Mode)
	d.Set("tier", model.ModelInfo.Tier)

	return nil
}
