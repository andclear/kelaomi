package main

// OpenAI API request/response structures

// ChatCompletionRequest represents the OpenAI chat completion request
type ChatCompletionRequest struct {
	Model       string                 `json:"model"`
	Messages    []ChatMessage          `json:"messages"`
	Temperature *float64               `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	MaxTokens   *int                   `json:"max_tokens,omitempty"`
	TopP        *float64               `json:"top_p,omitempty"`
	Stop        interface{}            `json:"stop,omitempty"`
	User        string                 `json:"user,omitempty"`
	Extra       map[string]interface{} `json:"-"`
}

// ChatMessage represents a single message in the conversation
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToOpenAIRequest 将自定义请求转换为标准OpenAI格式
func (r *ChatCompletionRequest) ToOpenAIRequest() ChatCompletionRequest {
	// 转换消息格式
	messages := make([]ChatMessage, len(r.Messages))
	for i, msg := range r.Messages {
		var content string
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []Content:
			for _, c := range v {
				content += c.Text
			}
		case []interface{}:
			for _, c := range v {
				if contentMap, ok := c.(map[string]interface{}); ok {
					if text, ok := contentMap["text"].(string); ok {
						content += text
					}
				}
			}
		}
		messages[i] = ChatMessage{
			Role:    msg.Role,
			Content: content,
		}
	}

	// 构建标准OpenAI请求格式
	return ChatCompletionRequest{
		Model:       r.Model,
		Temperature: r.Temperature,
		Messages:    messages,
		Stream:      r.Stream,
	}
}

// ChatCompletionResponse represents the OpenAI chat completion response
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

// ChatCompletionChoice represents a single choice in the response
type ChatCompletionChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason"`
}

// ChatCompletionUsage represents token usage information
type ChatCompletionUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

// ChatCompletionStreamResponse represents a streaming response chunk
type ChatCompletionStreamResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
}

// ModelsResponse represents the response for /v1/models endpoint
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model represents a single model in the models list
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// Atlassian API structures

// AtlassianRequest represents the request to Atlassian API
type AtlassianRequest struct {
	RequestPayload     AtlassianRequestPayload `json:"request_payload"`
	PlatformAttributes AtlassianPlatformAttrs  `json:"platform_attributes"`
}

// AtlassianRequestPayload represents the payload part of Atlassian request
type AtlassianRequestPayload struct {
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// AtlassianPlatformAttrs represents platform attributes for Atlassian API
type AtlassianPlatformAttrs struct {
	Model string `json:"model"`
}

// AtlassianResponse represents the response from Atlassian API
type AtlassianResponse struct {
	ResponsePayload    AtlassianResponsePayload `json:"response_payload"`
	PlatformAttributes AtlassianPlatformAttrs   `json:"platform_attributes"`
}

// AtlassianResponsePayload represents the payload part of Atlassian response
type AtlassianResponsePayload struct {
	ID      string                    `json:"id"`
	Created int64                     `json:"created"`
	Choices []AtlassianResponseChoice `json:"choices"`
}

// AtlassianResponseChoice represents a choice in Atlassian response
type AtlassianResponseChoice struct {
	Index        int                      `json:"index"`
	Message      AtlassianResponseMessage `json:"message"`
	FinishReason *string                  `json:"finish_reason"`
}

// AtlassianResponseMessage represents a message in Atlassian response
type AtlassianResponseMessage struct {
	Role    string                    `json:"role"`
	Content []AtlassianContentElement `json:"content"`
}

// AtlassianContentElement represents a content element in Atlassian message
type AtlassianContentElement struct {
	Text string `json:"text"`
}

// AtlassianMetrics represents usage metrics from Atlassian
type AtlassianMetrics struct {
	Usage ChatCompletionUsage `json:"usage"`
}

// AtlassianStreamChunk represents a streaming chunk from Atlassian
type AtlassianStreamChunk struct {
	ResponsePayload AtlassianResponsePayload `json:"response_payload"`
}
