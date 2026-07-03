package prompt

import "fmt"

// BuildTicketAnalysisPrompt constructs the Gemini prompt for ticket analysis.
// The prompt is engineered to return ONLY a JSON object with no markdown or prose.
func BuildTicketAnalysisPrompt(subject, description, customerName, category, priority string) string {
	return fmt.Sprintf(`You are an expert customer support AI analyst. Analyze the support ticket below.

Return ONLY a valid JSON object. No markdown. No code blocks. No explanations. Just the raw JSON.

Ticket Details:
Subject: %s
Description: %s
Customer: %s
Current Category: %s
Current Priority: %s

Required JSON structure (use EXACTLY these field names and allowed values):
{
  "category": "<one of: Payment, Authentication, Technical Issue, Refund, Account, Subscription, General>",
  "priority": "<one of: Low, Medium, High, Urgent>",
  "sentiment": "<one of: Positive, Neutral, Frustrated, Angry, Confused>",
  "recommended_team": "<one of: Finance, Support, Engineering, Sales, Security>",
  "confidence": <integer between 0 and 100>,
  "summary": "<single sentence describing the customer's issue>",
  "tags": ["<lowercase tag>", "<lowercase tag>", "<lowercase tag>"]
}

Rules:
- Output ONLY the JSON object. Nothing before or after it.
- confidence must be an integer (not a string).
- tags must be an array of 2 to 5 lowercase single-word strings.
- summary must be one sentence, under 150 characters.`, subject, description, customerName, category, priority)
}
