package dto

// GrammarExplainRequest contains the practice data already loaded by the client.
// SelectedOptionIndex is zero-based (A = 0, B = 1).
type GrammarExplainRequest struct {
	UnitTitle           string   `json:"unitTitle"`
	Question            string   `json:"question"`
	Options             []string `json:"options"`
	Correct             string   `json:"correct"`
	Explanation         string   `json:"explanation"`
	SelectedOptionIndex *int     `json:"selectedOptionIndex"`
}

type GrammarExplainResponse struct {
	Answer string `json:"answer"`
}
