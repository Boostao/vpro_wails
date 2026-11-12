package lifecycle

type StartupState struct {
	Ready          bool   `json:"ready"`
	Message        string `json:"message"`
	ConfigPath     string `json:"configPath"`
	ReleaseChannel string `json:"releaseChannel"`
}

type StartupService struct {
	State StartupState `json:"-" yaml:"-"`
}

func (s *StartupService) GetState() StartupState {
	return s.State
}
