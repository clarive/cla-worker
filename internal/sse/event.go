package sse

type Event struct {
	ID    string
	Event string
	Data  string
	Retry int
}
