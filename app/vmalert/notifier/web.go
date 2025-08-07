package notifier

type ApiNotifier struct {
	Kind    string       `json:"kind"`
	Targets []*ApiTarget `json:"targets"`
}

type ApiTarget struct {
	Address string            `json:"address"`
	Labels  map[string]string `json:"labels"`
	// LastError contains the error faced while sending to notifier.
	LastError string `json:"lastError"`
}
