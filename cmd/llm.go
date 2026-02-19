package cmd

import (
	"os"

	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/llm"
)

// newLLMClient creates an LLM client from config/env, or returns nil if no API key is configured.
func newLLMClient() *llm.Client {
	apiKey := viper.GetString("anthropic.api_key")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil
	}
	return llm.NewClient(apiKey, viper.GetString("anthropic.model"))
}
