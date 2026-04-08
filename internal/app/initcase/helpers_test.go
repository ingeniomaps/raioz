package initcase

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"raioz/internal/i18n"
)

func initI18n(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func newTestUseCase() *UseCase {
	return NewUseCase()
}

func newTestUseCaseWithIO(input string) (*UseCase, *bytes.Buffer) {
	in := strings.NewReader(input)
	out := &bytes.Buffer{}
	return &UseCase{
		In:     in,
		Out:    out,
		reader: bufio.NewReader(in),
	}, out
}
