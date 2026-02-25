package llm

// CollectStream drains a stream channel into a GenerateResponse.
// It blocks until the channel is closed.
func CollectStream(ch <-chan StreamEvent) GenerateResponse {
	var resp GenerateResponse
	var text string
	for ev := range ch {
		switch ev.Type {
		case StreamEventDelta:
			text += ev.Text
		case StreamEventComplete:
			if ev.Response != nil {
				resp = *ev.Response
			}
		}
	}
	// If no complete event was received, build response from accumulated text.
	if resp.StopReason == "" && text != "" {
		resp.Content = []ContentBlock{{Type: ContentTypeText, Text: text}}
		resp.StopReason = StopReasonEndTurn
	}
	return resp
}
