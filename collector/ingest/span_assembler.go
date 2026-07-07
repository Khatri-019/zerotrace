package ingest

// SpanAssembler groups spans into complete traces
type SpanAssembler struct {}

func NewSpanAssembler() *SpanAssembler {
	return &SpanAssembler{}
}
