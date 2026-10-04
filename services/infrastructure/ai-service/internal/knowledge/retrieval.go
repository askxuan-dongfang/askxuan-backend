package knowledge

import "context"

// RetrievalPolicy is frozen with the agent version, including debug/evaluation.
type RetrievalPolicy struct {
	CandidateCount int    `json:"candidateCount"`
	ResultCount    int    `json:"resultCount"`
	RerankModel    string `json:"rerankModel"`
	RerankRequired bool   `json:"rerankRequired"`
	GraphEnabled   bool   `json:"graphEnabled"`
}
type retrievalKey struct{}

func WithRetrieval(ctx context.Context, p RetrievalPolicy) context.Context {
	return context.WithValue(ctx, retrievalKey{}, p)
}
func RetrievalFrom(ctx context.Context) RetrievalPolicy {
	p, _ := ctx.Value(retrievalKey{}).(RetrievalPolicy)
	if p.CandidateCount == 0 {
		p.CandidateCount = 12
	}
	if p.ResultCount == 0 {
		p.ResultCount = 5
	}
	return p
}
func (p RetrievalPolicy) Valid() bool {
	return (p.CandidateCount == 0 || p.CandidateCount >= 5 && p.CandidateCount <= 40) && (p.ResultCount == 0 || p.ResultCount >= 1 && p.ResultCount <= 10) && (p.CandidateCount == 0 || p.ResultCount <= p.CandidateCount) && len(p.RerankModel) <= 64 && (!p.RerankRequired || p.RerankModel != "")
}
