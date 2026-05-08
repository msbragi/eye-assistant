package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// LLMClient communicates with the llama-server OpenAI-compatible API.
type LLMClient struct {
	baseURL string
}

func NewLLMClient(port string) *LLMClient {
	return &LLMClient{baseURL: "http://127.0.0.1:" + port}
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ContentPart
}

// ContentPart is used for multimodal messages (text + image).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"` // data:image/jpeg;base64,...
}

// ChatRequest is sent to llama-server /v1/chat/completions.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// StreamChunk is one SSE chunk from the streaming response.
type StreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

const systemPrompt = `You are a helpful home assistant. The user will show you images of household objects, documents, appliances, or screens.
Your job is to explain clearly and concisely what you see, answer any question about it, and give practical advice.
Use simple language. Be direct. If the image shows a document (bill, letter, manual), summarise the key information.
If the image shows an appliance or device, explain the controls and symbols in plain terms.
Always respond in the same language the user writes in.`

// AskWithImage sends an image + question to llama-server and streams the response
// back by calling onChunk for each text token received.
func (c *LLMClient) AskWithImage(imagePath, question string, onChunk func(string)) error {
	imageData, err := encodeImageBase64(imagePath)
	if err != nil {
		return fmt.Errorf("encoding image: %w", err)
	}

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{
			Role: "user",
			Content: []ContentPart{
				{
					Type: "image_url",
					ImageURL: &ImageURL{
						URL: "data:image/jpeg;base64," + imageData,
					},
				},
				{Type: "text", Text: question},
			},
		},
	}

	return c.streamChat(messages, onChunk)
}

// AskText sends a plain text question (no image) and streams the response.
func (c *LLMClient) AskText(question string, onChunk func(string)) error {
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}
	return c.streamChat(messages, onChunk)
}

func (c *LLMClient) streamChat(messages []ChatMessage, onChunk func(string)) error {
	reqBody := ChatRequest{
		Model:    "gemma",
		Messages: messages,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("calling llama-server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llama-server returned %d: %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk StreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	return scanner.Err()
}

func encodeImageBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
