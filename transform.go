package main

import (
	"strings"
	"time"
)

// TransformModelID removes vendor prefix (e.g. "anthropic:")
func TransformModelID(modelID string) string {
	parts := strings.Split(modelID, ":")
	return parts[len(parts)-1]
}

func ToOpenAI(atlasResp AtlassianResponse, modelID string) ChatCompletionResponse {

	var usage ChatCompletionUsage
	if atlasResp.PlatformAttributes.Model != "" {

		usage = ChatCompletionUsage{
			PromptTokens:     nil,
			CompletionTokens: nil,
			TotalTokens:      nil,
		}
	}

	// Convert choices
	choices := make([]ChatCompletionChoice, len(atlasResp.ResponsePayload.Choices))
	for i, choice := range atlasResp.ResponsePayload.Choices {
		// Extract text content from the first content element
		var content string
		if len(choice.Message.Content) > 0 {
			content = choice.Message.Content[0].Text
		}

		choices[i] = ChatCompletionChoice{
			Index: choice.Index,
			Message: &ChatMessage{
				Role:    choice.Message.Role,
				Content: content,
			},
			FinishReason: choice.FinishReason,
		}
	}

	return ChatCompletionResponse{
		ID:      atlasResp.ResponsePayload.ID,
		Object:  "chat.completion",
		Created: atlasResp.ResponsePayload.Created,
		Model:   modelID,
		Choices: choices,
		Usage:   usage,
	}
}

// ToOpenAIStreamChunk converts Atlassian stream chunk to OpenAI format
func ToOpenAIStreamChunk(atlasChunk AtlassianStreamChunk, requestedModel string) ChatCompletionStreamResponse {
	var choices []ChatCompletionChoice

	if len(atlasChunk.ResponsePayload.Choices) > 0 {
		choice := atlasChunk.ResponsePayload.Choices[0]

		delta := &ChatMessage{}

		// Set role if present
		if choice.Message.Role != "" {
			delta.Role = choice.Message.Role
		}

		// Extract text content
		if len(choice.Message.Content) > 0 && choice.Message.Content[0].Text != "" {
			delta.Content = choice.Message.Content[0].Text
		}

		// Only add choice if there's meaningful content or finish reason
		if delta.Role != "" || delta.Content != "" || choice.FinishReason != nil {
			choices = append(choices, ChatCompletionChoice{
				Index:        choice.Index,
				Delta:        delta,
				FinishReason: choice.FinishReason,
			})
		}
	}

	// Generate ID if not present
	id := atlasChunk.ResponsePayload.ID
	if id == "" {
		id = generateChatCompletionID()
	}

	// Use created time if present, otherwise current time
	created := atlasChunk.ResponsePayload.Created
	if created == 0 {
		created = time.Now().Unix()
	}

	return ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   requestedModel,
		Choices: choices,
	}
}

// generateChatCompletionID generates a chat completion ID similar to OpenAI format
func generateChatCompletionID() string {
	return "chatcmpl-" + string(rune(time.Now().UnixMilli()))
}
