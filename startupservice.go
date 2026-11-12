package main

type StartupState struct {
	Ready          bool   `json:"ready"`
	Message        string `json:"message"`
	ConfigPath     string `json:"configPath"`
	ReleaseChannel string `json:"releaseChannel"`
}

type StartupService struct {
	state StartupState
}

func (s *StartupService) GetState() StartupState {
	return s.state
}
