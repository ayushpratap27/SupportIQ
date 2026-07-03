package replyprompt

import (
	"fmt"
	"strings"

	replyprovider "github.com/ayush/supportiq/internal/ai/reply/provider"
)

// CurrentVersion is the prompt template version. Bump when the prompt changes.
const CurrentVersion = "v1"

// BuildReplyPrompt constructs the RAG-enhanced Gemini prompt for reply generation.
// It embeds relevant knowledge base documents so the AI answers only from
// verified company policies — never from its own pre-trained knowledge.
func BuildReplyPrompt(req replyprovider.ReplyRequest) string {
	var sb strings.Builder

	sb.WriteString("You are a professional customer support AI assistant. " +
		"Your task is to draft a reply to a customer support ticket.\n\n")
	sb.WriteString("Return ONLY a valid JSON object. No markdown. No code blocks. " +
		"No explanations. Just the raw JSON.\n\n")

	sb.WriteString("--- TICKET DETAILS ---\n")
	sb.WriteString(fmt.Sprintf("Subject: %s\n", req.Subject))
	sb.WriteString(fmt.Sprintf("Description: %s\n", req.Description))
	sb.WriteString(fmt.Sprintf("Category: %s\n", req.Category))
	sb.WriteString(fmt.Sprintf("Priority: %s\n", req.Priority))
	sb.WriteString(fmt.Sprintf("Customer Sentiment: %s\n\n", req.Sentiment))

	sb.WriteString("--- RELEVANT KNOWLEDGE BASE DOCUMENTS ---\n\n")
	for i, doc := range req.Documents {
		sb.WriteString(fmt.Sprintf("[Document %d] %s (%s)\n", i+1, doc.Title, doc.Category))
		sb.WriteString(doc.Content)
		sb.WriteString("\n\n")
	}
	sb.WriteString("--- END OF KNOWLEDGE BASE DOCUMENTS ---\n\n")

	sb.WriteString(`Instructions:
- Write a professional, empathetic customer support reply.
- Be concise and solution-focused.
- NEVER invent company policies. Use ONLY the information from the documents above.
- If the documents do not fully address the issue, acknowledge the concern and state that you will follow up.
- Do not include subject lines or greetings like "Dear Customer" — go straight to the response body.

Required JSON structure (use EXACTLY these field names):
{
  "reply": "<the complete ready-to-send reply text>",
  "confidence": <integer between 0 and 100>
}

Rules:
- Output ONLY the JSON object. Nothing before or after it.
- confidence must be an integer reflecting how well the knowledge base covers the issue (100 = perfect match).
- reply must be a complete, professional customer support response.`)

	return sb.String()
}
