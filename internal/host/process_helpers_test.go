package host

import (
	"os"
	"reflect"
	"testing"
)

func statLink(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func TestParseCommand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single word", "npm", []string{"npm"}},
		{"with args", "npm run dev", []string{"npm", "run", "dev"}},
		{"multiple spaces", "go  run  main.go", []string{"go", "run", "main.go"}},
		{"tabs and spaces", "make\tlaunch", []string{"make", "launch"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCommand(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCommand(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestShouldWaitForCommand(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    bool
	}{
		{"make launch", "make launch", true},
		{"make stop", "make stop", true},
		{"docker-compose up", "docker-compose up -d", true},
		{"docker compose up", "docker compose up", true},
		{"docker-compose down", "docker-compose down", true},
		{"docker compose down", "docker compose down", true},
		{"npm run dev", "npm run dev", false},
		{"go run", "go run main.go", false},
		{"empty", "", false},
		{"setup script sh prefix", "sh setup.sh", true},
		{"script with .sh suffix", "./installer.sh", true},
		{"relative binary", "./mybinary", true},
		{"go run script-like", "go run script.sh", false},
		{"python", "python main.py", false},
		{"node", "node index.js", false},
		{"case insensitive", "MAKE LAUNCH", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldWaitForCommand(tc.command)
			if got != tc.want {
				t.Errorf("shouldWaitForCommand(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestParseEnvFile(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"empty", "", nil},
		{"only comments", "# comment\n# another", nil},
		{"simple vars", "FOO=bar\nBAZ=qux", []string{"FOO=bar", "BAZ=qux"}},
		{"with empty lines", "FOO=bar\n\nBAZ=qux\n", []string{"FOO=bar", "BAZ=qux"}},
		{"with comments", "# comment\nFOO=bar\n# another\nBAZ=qux", []string{"FOO=bar", "BAZ=qux"}},
		{"with whitespace", "  FOO=bar  \n  BAZ=qux  ", []string{"FOO=bar", "BAZ=qux"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseEnvFile(tc.content)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseEnvFile(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestCreateVolumeSymlinksInvalidFormat(t *testing.T) {
	cases := []struct {
		name    string
		volumes []string
		wantErr bool
	}{
		{"missing colon", []string{"invalid"}, true},
		{"empty src", []string{":dest"}, true},
		{"empty dest", []string{"src:"}, true},
		{"empty string skipped", []string{""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			err := createVolumeSymlinks(tc.volumes, dir, dir)
			if (err != nil) != tc.wantErr {
				t.Errorf("createVolumeSymlinks() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCreateVolumeSymlinksDirectory(t *testing.T) {
	project := t.TempDir()
	service := t.TempDir()

	err := createVolumeSymlinks([]string{"shared:./linked"}, project, service)
	if err != nil {
		t.Fatalf("createVolumeSymlinks() error = %v", err)
	}

	// Verify symlink was created
	link := service + "/linked"
	if _, err := statLink(link); err != nil {
		t.Errorf("symlink not created: %v", err)
	}
}

func TestCreateVolumeSymlinksFile(t *testing.T) {
	project := t.TempDir()
	service := t.TempDir()

	err := createVolumeSymlinks([]string{"config.json:./app.json"}, project, service)
	if err != nil {
		t.Fatalf("createVolumeSymlinks() error = %v", err)
	}

	link := service + "/app.json"
	if _, err := statLink(link); err != nil {
		t.Errorf("symlink not created: %v", err)
	}
}

func TestCreateVolumeSymlinksIdempotent(t *testing.T) {
	project := t.TempDir()
	service := t.TempDir()

	// Run twice - should not error
	if err := createVolumeSymlinks([]string{"data:./data"}, project, service); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := createVolumeSymlinks([]string{"data:./data"}, project, service); err != nil {
		t.Errorf("second call: %v", err)
	}
}

func TestCreateVolumeSymlinksRelativeNoProjectDir(t *testing.T) {
	service := t.TempDir()
	err := createVolumeSymlinks([]string{"rel:./dest"}, "", service)
	if err == nil {
		t.Errorf("expected error when projectDir empty and src relative")
	}
}
