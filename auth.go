package main

import (
	"encoding/base64"
	"fmt"
)

func AuthHeaders(email, apiToken string) map[string]string {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", email, apiToken)))
	return map[string]string{
		"Content-Type":             "application/json",
		"Accept":                   "application/json",
		"Authorization":            fmt.Sprintf("Basic %s", encoded),
		"X-Atlassian-EncodedToken": encoded,
	}
}
