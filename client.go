package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-resty/resty/v2"
)

// HTTPClient wraps resty client with retry logic
type HTTPClient struct {
	client *resty.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient() *HTTPClient {
	client := resty.New()
	client.SetTimeout(0) // No timeout for streaming
	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	return &HTTPClient{
		client: client,
	}
}

// FetchWithRetry performs HTTP request with credential rotation and exponential backoff
func (c *HTTPClient) FetchWithRetry(ctx context.Context, body AtlassianRequest, stream bool) (*resty.Response, error) {
	delay := InitialDelay
	attempts := 0
	credIdx := 0

	for attempts < len(Credentials) {
		cred := Credentials[credIdx]
		headers := AuthHeaders(cred.Email, cred.Token)

		req := c.client.R().
			SetContext(ctx).
			SetBody(body)

		for key, value := range headers {
			req.SetHeader(key, value)
		}

		if stream {
			req.SetDoNotParseResponse(true)
		}

		resp, err := req.Post(AtlassianAPIEndpoint)

		if err == nil && resp.StatusCode() < 400 {
			return resp, nil
		}

		if DebugMode {
			if err != nil {
				log.Printf("Request error using credential #%d: %v", credIdx, err)
			} else {
				log.Printf("Credential #%d failed (status %d). Retrying…", credIdx, resp.StatusCode())
			}
		}

		if err != nil || resp.StatusCode() == 401 || resp.StatusCode() == 403 || resp.StatusCode() >= 500 {

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			delay = time.Duration(float64(delay) * DelayMultiplier)
			if delay > MaxDelay {
				delay = MaxDelay
			}

			credIdx = (credIdx + 1) % len(Credentials)
			attempts++
		} else {

			return resp, fmt.Errorf("non-retryable error: status %d", resp.StatusCode())
		}
	}

	return nil, fmt.Errorf("all credentials exhausted after %d attempts", attempts)
}

type StreamResponse struct {
	Response *resty.Response
	Model    string
}

func (sr *StreamResponse) StreamLines(ctx context.Context) (<-chan []byte, <-chan error) {
	linesChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(linesChan)
		defer close(errChan)
		defer sr.Response.RawBody().Close()

		buffer := make([]byte, 4096)
		var accumulated []byte

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			n, err := sr.Response.RawBody().Read(buffer)
			if n > 0 {
				accumulated = append(accumulated, buffer[:n]...)

				// Process complete lines
				for {
					lineEnd := -1
					for i := 0; i < len(accumulated)-1; i++ {
						if accumulated[i] == '\n' && accumulated[i+1] == '\n' {
							lineEnd = i + 2
							break
						}
					}

					if lineEnd == -1 {
						break
					}

					line := accumulated[:lineEnd-2] // Remove \n\n
					accumulated = accumulated[lineEnd:]

					if len(line) > 0 {
						select {
						case linesChan <- line:
						case <-ctx.Done():
							errChan <- ctx.Err()
							return
						}
					}
				}
			}

			if err != nil {
				if err.Error() != "EOF" {
					errChan <- err
				}
				return
			}
		}
	}()

	return linesChan, errChan
}

func (sr *StreamResponse) ConvertToOpenAIStream(ctx context.Context) (<-chan []byte, <-chan error) {
	outputChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	linesChan, inputErrChan := sr.StreamLines(ctx)

	go func() {
		defer close(outputChan)
		defer close(errChan)

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case err := <-inputErrChan:
				if err != nil {
					errChan <- err
					return
				}
			case line, ok := <-linesChan:
				if !ok {
					// Send final [DONE] message
					select {
					case outputChan <- []byte("data: [DONE]\n\n"):
					case <-ctx.Done():
						errChan <- ctx.Err()
					}
					return
				}

				lineStr := string(line)
				if !hasPrefix(lineStr, "data:") {
					continue
				}

				data := trim(lineStr[5:])
				if data == "[DONE]" {
					continue
				}

				// Parse Atlassian chunk
				var atlasChunk AtlassianStreamChunk
				if err := json.Unmarshal([]byte(data), &atlasChunk); err != nil {
					if DebugMode {
						log.Printf("Unable to decode JSON from upstream: %s", data[:min(len(data), 100)])
					}
					continue
				}

				// Convert to OpenAI format
				openChunk := ToOpenAIStreamChunk(atlasChunk, sr.Model)

				// Skip empty chunks
				if len(openChunk.Choices) == 0 {
					continue
				}

				choice := openChunk.Choices[0]
				if choice.Delta == nil || (choice.Delta.Role == "" && choice.Delta.Content == "" && choice.FinishReason == nil) {
					continue
				}

				chunkBytes, err := json.Marshal(openChunk)
				if err != nil {
					errChan <- err
					return
				}

				sseData := fmt.Sprintf("data: %s\n\n", string(chunkBytes))
				select {
				case outputChan <- []byte(sseData):
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				}
			}
		}
	}()

	return outputChan, errChan
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trim(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
