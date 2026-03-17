package pubsub

type Message struct {
	OID    string
	Event  string
	Data   map[string]interface{}
}

type EventData struct {
	OID    string                            `json:"oid"`
	Events map[string][]map[string]interface{} `json:"events"`
}
