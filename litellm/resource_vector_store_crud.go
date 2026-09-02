package litellm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLiteLLMVectorStoreCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)

	vectorStoreName := d.Get("vector_store_name").(string)
	customLLMProvider := d.Get("custom_llm_provider").(string)
	vectorStoreDescription := d.Get("vector_store_description").(string)
	vectorStoreMetadata := d.Get("vector_store_metadata").(map[string]interface{})
	litellmCredentialName := d.Get("litellm_credential_name").(string)
	litellmParams := d.Get("litellm_params").(map[string]interface{})

	// Generate a vector_store_id if not provided
	vectorStoreID := d.Get("vector_store_id").(string)
	if vectorStoreID == "" {
		vectorStoreID = uuid.New().String()
	}

	// Convert metadata to map[string]interface{} for JSON
	metadataMap := make(map[string]interface{})
	for k, v := range vectorStoreMetadata {
		metadataMap[k] = v
	}

	// Convert litellm_params to map[string]interface{} for JSON
	paramsMap := make(map[string]interface{})
	for k, v := range litellmParams {
		paramsMap[k] = v
	}

	vectorStoreRequest := VectorStoreRequest{
		VectorStoreID:          vectorStoreID,
		CustomLLMProvider:      customLLMProvider,
		VectorStoreName:        vectorStoreName,
		VectorStoreDescription: vectorStoreDescription,
		VectorStoreMetadata:    metadataMap,
		LiteLLMCredentialName:  litellmCredentialName,
		LiteLLMParams:          paramsMap,
	}

	resp, err := MakeRequest(client, "POST", "/vector_store/new", vectorStoreRequest)
	if err != nil {
		return fmt.Errorf("failed to create vector store: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create vector store: Status: %s, Response: %s",
			resp.Status, client.redactSensitiveData(string(bodyBytes)))
	}

	var createResp struct {
		VectorStore *VectorStoreResponse `json:"vector_store"`
		VectorStoreResponse
	}
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		return fmt.Errorf("failed to parse create response: %v", err)
	}

	respVS := createResp.VectorStoreResponse
	if createResp.VectorStore != nil {
		respVS = *createResp.VectorStore
	}

	if respVS.VectorStoreID != "" {
		d.SetId(respVS.VectorStoreID)
	} else {
		d.SetId(vectorStoreID)
	}

	d.Set("vector_store_id", respVS.VectorStoreID)
	d.Set("vector_store_name", respVS.VectorStoreName)
	d.Set("custom_llm_provider", respVS.CustomLLMProvider)
	d.Set("vector_store_description", respVS.VectorStoreDescription)
	d.Set("vector_store_metadata", respVS.VectorStoreMetadata)
	d.Set("litellm_credential_name", respVS.LiteLLMCredentialName)
	d.Set("created_at", respVS.CreatedAt)
	d.Set("updated_at", respVS.UpdatedAt)

	return nil
}

func resourceLiteLLMVectorStoreRead(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	vectorStoreID := d.Id()

	// Use the info endpoint to get vector store details
	infoRequest := VectorStoreInfoRequest{
		VectorStoreID: vectorStoreID,
	}

	resp, err := MakeRequest(client, "POST", "/vector_store/info", infoRequest)
	if err != nil {
		return fmt.Errorf("failed to read vector store: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}

	// The API wraps the response in {"vector_store": {...}}; fall back to a
	// flat response for robustness.
	var wrapper struct {
		VectorStore *VectorStoreResponse `json:"vector_store"`
		VectorStoreResponse
	}
	err = handleVectorStoreAPIResponse(resp, &wrapper, client)
	if err != nil {
		if err.Error() == "vector_store_not_found" {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read vector store: %w", err)
	}

	vectorStoreResp := wrapper.VectorStoreResponse
	if wrapper.VectorStore != nil {
		vectorStoreResp = *wrapper.VectorStore
	}

	// Update the resource ID to the actual vector store ID from the response
	if vectorStoreResp.VectorStoreID != "" {
		d.SetId(vectorStoreResp.VectorStoreID)
	}

	d.Set("vector_store_id", vectorStoreResp.VectorStoreID)
	d.Set("vector_store_name", vectorStoreResp.VectorStoreName)
	d.Set("custom_llm_provider", vectorStoreResp.CustomLLMProvider)
	d.Set("vector_store_description", vectorStoreResp.VectorStoreDescription)
	d.Set("vector_store_metadata", vectorStoreResp.VectorStoreMetadata)
	d.Set("litellm_credential_name", vectorStoreResp.LiteLLMCredentialName)
	d.Set("created_at", vectorStoreResp.CreatedAt)
	d.Set("updated_at", vectorStoreResp.UpdatedAt)

	return nil
}

func resourceLiteLLMVectorStoreUpdate(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	vectorStoreID := d.Id()

	vectorStoreName := d.Get("vector_store_name").(string)
	customLLMProvider := d.Get("custom_llm_provider").(string)
	vectorStoreDescription := d.Get("vector_store_description").(string)
	vectorStoreMetadata := d.Get("vector_store_metadata").(map[string]interface{})

	// Convert metadata to map[string]interface{} for JSON
	metadataMap := make(map[string]interface{})
	for k, v := range vectorStoreMetadata {
		metadataMap[k] = v
	}

	vectorStoreRequest := VectorStoreRequest{
		VectorStoreID:          vectorStoreID,
		CustomLLMProvider:      customLLMProvider,
		VectorStoreName:        vectorStoreName,
		VectorStoreDescription: vectorStoreDescription,
		VectorStoreMetadata:    metadataMap,
	}

	resp, err := MakeRequest(client, "POST", "/vector_store/update", vectorStoreRequest)
	if err != nil {
		return fmt.Errorf("failed to update vector store: %w", err)
	}
	defer resp.Body.Close()

	err = handleVectorStoreAPIResponse(resp, nil, client)
	if err != nil {
		return fmt.Errorf("failed to update vector store: %w", err)
	}

	return resourceLiteLLMVectorStoreRead(d, m)
}

func resourceLiteLLMVectorStoreDelete(d *schema.ResourceData, m interface{}) error {
	client := m.(*Client)
	vectorStoreID := d.Id()

	deleteRequest := VectorStoreDeleteRequest{
		VectorStoreID: vectorStoreID,
	}

	resp, err := MakeRequest(client, "POST", "/vector_store/delete", deleteRequest)
	if err != nil {
		return fmt.Errorf("failed to delete vector store: %w", err)
	}
	defer resp.Body.Close()

	err = handleVectorStoreAPIResponse(resp, nil, client)
	if err != nil {
		if err.Error() == "vector_store_not_found" {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to delete vector store: %w", err)
	}

	d.SetId("")
	return nil
}
