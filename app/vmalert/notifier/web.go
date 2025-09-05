package notifier

// ApiNotifier represents a Notifier configuration for WEB view
type ApiNotifier struct {
	// Kind is a Notifier type
	Kind TargetType `json:"kind"`
	// Targets is a list of Notifier targets
	Targets []*ApiTarget `json:"targets"`
}

// ApiTarget represents a specific Notifier target for WEB view
type ApiTarget struct {
	// Address is a URL for sending notifications
	Address string `json:"address"`
	// Labels is a list of labels to add to each sent notification
	Labels map[string]string `json:"labels"`
	// LastError contains the error faced while sending to notifier.
	LastError string `json:"lastError"`
}
