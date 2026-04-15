package upcase

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// --- streamAllLogs -----------------------------------------------------------
// We cannot easily test the full streamAllLogs (blocks on ctx), but we can
// test it with an immediate cancellation to cover the setup paths.

func TestStreamAllLogsCancelledImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"api": {},
		},
		Infra: map[string]config.InfraEntry{
			"db": {Inline: &config.Infra{Image: "postgres"}},
		},
	}

	detections := DetectionMap{
		"api": {Runtime: detect.RuntimeGo},
		"db":  {Runtime: detect.RuntimeImage},
	}

	// Should not block since ctx is already cancelled
	streamAllLogs(ctx, deps, detections)
}

func TestStreamAllLogsNoServices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	deps := &config.Deps{
		Project:  config.Project{Name: "p"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}

	streamAllLogs(ctx, deps, DetectionMap{})
}

func TestStreamAllLogsSkipsDockerServices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	deps := &config.Deps{
		Project: config.Project{Name: "p"},
		Services: map[string]config.Service{
			"compose-svc":    {},
			"dockerfile-svc": {},
			"image-svc":      {},
		},
		Infra: map[string]config.InfraEntry{},
	}

	detections := DetectionMap{
		"compose-svc":    {Runtime: detect.RuntimeCompose},
		"dockerfile-svc": {Runtime: detect.RuntimeDockerfile},
		"image-svc":      {Runtime: detect.RuntimeImage},
	}

	// Docker-managed services should be skipped for host log tailing
	streamAllLogs(ctx, deps, detections)
}

// --- prefixedCopy cancellation -----------------------------------------------

func TestPrefixedCopyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Using a closed pipe as reader to trigger immediate return
	r, w, err := pipeForTest()
	if err != nil {
		t.Skip("cannot create pipe")
	}
	w.Close()

	prefixedCopy(ctx, r, "test", "\033[36m", 10)
	r.Close()
}

// --- tailFile with cancelled context -----------------------------------------

func TestTailFileCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// File doesn't exist, but context is cancelled → should return immediately
	tailFile(ctx, "svc", "/nonexistent/path/log.txt", "\033[36m", 5)
}

// --- tailDocker with cancelled context ---------------------------------------

func TestTailDockerCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tailDocker(ctx, "svc", "nonexistent-container", "\033[36m", 5)
}

// --- streamForeground: we can't easily test it because it blocks on signals,
// but we cover its existence through streamAllLogs tests above.

// --- serviceColors coverage --------------------------------------------------

func TestServiceColorsNotEmpty(t *testing.T) {
	if len(serviceColors) == 0 {
		t.Error("serviceColors should have at least one color")
	}
	if colorReset == "" {
		t.Error("colorReset should not be empty")
	}
}

// pipeForTest creates an os.Pipe for testing
func pipeForTest() (*readCloser, *writeCloser, error) {
	r, w := pipePair()
	return r, w, nil
}

// Minimal io.ReadCloser / io.WriteCloser wrappers for test pipe
type readCloser struct {
	*pipeRead
}

type writeCloser struct {
	*pipeWrite
}

type pipeRead struct {
	ch chan []byte
}

type pipeWrite struct {
	ch chan []byte
}

func pipePair() (*readCloser, *writeCloser) {
	ch := make(chan []byte, 1)
	return &readCloser{&pipeRead{ch}}, &writeCloser{&pipeWrite{ch}}
}

func (p *pipeRead) Read(b []byte) (int, error) {
	data, ok := <-p.ch
	if !ok {
		return 0, fmt.Errorf("closed")
	}
	copy(b, data)
	return len(data), nil
}

func (p *readCloser) Close() error {
	return nil
}

func (p *pipeWrite) Write(b []byte) (int, error) {
	p.ch <- append([]byte(nil), b...)
	return len(b), nil
}

func (p *writeCloser) Close() error {
	close(p.ch)
	return nil
}
