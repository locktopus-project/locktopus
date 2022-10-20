package client

// This definitions are copied from cmd/server/api_v1.go.

type action string

const (
	actionLock    action = "lock"
	actionRelease action = "release"
)

type requestMessage struct {
	Action    action     `json:"action"`
	Resources []resource `json:"resources,omitempty"`
}

type resource struct {
	T    string   `json:"type"`
	Path []string `json:"path"`
}

type responseMessage struct {
	ID     string `json:"id"`
	Action action `json:"action"`
	State  string `json:"state"`
}
