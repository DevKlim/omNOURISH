package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages[]OpenAIMessage `json:"messages"`
}

type OpenAIResponse struct {
	Choices[]struct {
		Message OpenAIMessage `json:"message"`
	} `json:"choices"`
}

func (a *App) callLLM(userMessage, apiKey, model, provider, baseUrl string) string {
	token := apiKey
	if token == "" {
		token = os.Getenv("NRP_API_TOKEN")
	}

	if token == "" || token == "your_nrp_token_here" {
		return "**Missing API Token**: Please configure your API Token in the Agent Settings."
	}

	systemPrompt := `You are the Nourish PT Data Agent. You help users analyze the San Diego food map using the L, E, M, P, and T indicator framework.`

	url := baseUrl
	if url == "" {
		if provider == "NRP" {
			url = "https://ellm.nrp-nautilus.io/v1/chat/completions"
		} else {
			url = "https://api.openai.com/v1/chat/completions"
		}
	}

	openAiReq := OpenAIRequest{
		Model: "gpt-oss",
		Messages:[]OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	jsonData, _ := json.Marshal(openAiReq)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error connecting to LLM endpoint: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
		return "Failed to parse LLM response format."
	}

	if len(openAIResp.Choices) > 0 {
		return openAIResp.Choices[0].Message.Content
	}
	return "LLM returned an empty response."
}

func (a *App) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	replyText := a.callLLM(req.Message, req.ApiKey, req.Model, req.Provider, req.BaseUrl)
	resp := MapConfigResponse{
		Reply:           replyText,
		ActiveLayers:[]string{"Agent Modifiers Enabled"},
		ActiveWorkspace: "LLM Analyzed View",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleExploreDB(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		tableName = "nourish_cbg_food_environment"
	}

	if a.DB == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusInternalServerError)
		return
	}

	query := `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`
	rows, err := a.DB.Query(query, tableName)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []map[string]string
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err == nil {
			columns = append(columns, map[string]string{"column": colName, "type": dataType})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"table": tableName, "columns": columns})
}
