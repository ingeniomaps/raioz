package mocks

type DockerRunner struct {
	UpFunc   func(composePath string) error
	DownFunc func(composePath string) error
}

func (m *DockerRunner) Up(composePath string) error {
	if m.UpFunc != nil {
		return m.UpFunc(composePath)
	}
	return nil
}

func (m *DockerRunner) Down(composePath string) error {
	if m.DownFunc != nil {
		return m.DownFunc(composePath)
	}
	return nil
}
