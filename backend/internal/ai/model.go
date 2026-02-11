package ai

// GetModelUsed returns the model identifier used for a request.
// It prefers response metadata, then request model, then provider name.
func GetModelUsed(resp *AIResponse, req *AIRequest) string {
	if resp != nil && resp.Metadata != nil {
		if v, ok := resp.Metadata["model"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if req != nil && req.Model != "" {
		return req.Model
	}
	if resp != nil && resp.Provider != "" {
		return string(resp.Provider)
	}
	return "unknown"
}
