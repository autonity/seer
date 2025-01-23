package model

// EventSchema is the combination of abi schema and influx db schema, inf future this could be decoupled into separate event objectsg
type EventSchema struct {
	Measurement string                 `json:"event_name"`
	Fields      map[string]interface{} `json:"fields"`
	Tags        map[string]string
}

type JSONRPCRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  map[string]interface{} `json:"result"`
	Error   *JSONRPCError          `json:"error,omitempty"`
}

// JSONRPCError represents an error in a JSON-RPC response
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
