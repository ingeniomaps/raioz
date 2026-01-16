package git

import (
	"strings"
	"testing"
)

func TestValidateBranch(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		// Valid branches
		{"valid simple branch", "main", false},
		{"valid branch with hyphen", "feature-branch", false},
		{"valid branch with underscore", "feature_branch", false},
		{"valid branch with slash", "feature/branch", false},
		{"valid branch with refs prefix", "refs/heads/main", false},
		{"valid branch with dots", "v1.0.0", false},
		{"valid complex branch", "feature/user-auth-v2", false},

		// Invalid branches - dangerous characters
		{"branch with semicolon", "main; rm -rf", true},
		{"branch with pipe", "main | cat", true},
		{"branch with ampersand", "main & echo", true},
		{"branch with dollar", "main $HOME", true},
		{"branch with backtick", "main `ls`", true},
		{"branch with newline", "main\nrm -rf", true},
		{"branch with carriage return", "main\rm", true},
		{"branch with tab", "main\trm", true},

		// Invalid branches - empty
		{"empty branch", "", true},

		// Invalid branches - too long
		{"branch too long", strings.Repeat("a", 256), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBranch(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBranch(%q) error = %v, wantErr %v", tt.branch, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRepo(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		// Valid repositories
		{"valid https URL", "https://github.com/user/repo.git", false},
		{"valid http URL", "http://github.com/user/repo.git", false},
		{"valid SSH URL", "ssh://git@github.com/user/repo.git", false},
		{"valid git@ URL", "git@github.com:user/repo.git", false},
		{"valid file URL", "file:///path/to/repo.git", false},
		{"valid git@ with path", "git@github.com:user/project/repo.git", false},

		// Invalid repositories - dangerous characters
		{"repo with semicolon", "https://github.com/user/repo.git; rm -rf", true},
		{"repo with pipe", "git@github.com:user/repo.git | cat", true},
		{"repo with ampersand", "https://github.com/user/repo.git & echo", true},
		{"repo with dollar", "git@github.com:user/repo.git $HOME", true},
		{"repo with backtick", "https://github.com/user/repo.git `ls`", true},
		{"repo with newline", "git@github.com:user/repo.git\nrm -rf", true},
		{"repo with carriage return", "https://github.com/user/repo.git\rm", true},
		{"repo with tab", "git@github.com:user/repo.git\trm", true},

		// Invalid repositories - empty
		{"empty repo", "", true},

		// Invalid repositories - invalid format
		{"invalid git@ format (no colon)", "git@github.com/user/repo.git", true},
		{"invalid git@ format (multiple colons)", "git@github.com:user:repo.git", true},
		{"invalid protocol", "invalid://github.com/user/repo.git", true},

		// Invalid repositories - too long
		{"repo too long", "https://github.com/" + strings.Repeat("a", 2040) + ".git", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRepo(%q) error = %v, wantErr %v", tt.repo, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{"valid simple path", "/path/to/repo", false},
		{"valid relative path", "../repo", false},
		{"valid path with dots", "./repo", false},
		{"valid path with spaces", "/path/to my repo", false},
		{"valid Windows path", "C:\\path\\to\\repo", false},

		// Invalid paths - dangerous characters
		{"path with semicolon", "/path/to/repo; rm -rf", true},
		{"path with pipe", "/path/to/repo | cat", true},
		{"path with ampersand", "/path/to/repo & echo", true},
		{"path with dollar", "/path/to/repo $HOME", true},
		{"path with backtick", "/path/to/repo `ls`", true},
		{"path with newline", "/path/to/repo\nrm -rf", true},
		{"path with carriage return", "/path/to/repo\rm", true},
		{"path with tab", "/path/to/repo\trm", true},
		{"path with null byte", "/path/to/repo\x00", true},

		// Invalid paths - empty
		{"empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateGitInput(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		repo    string
		wantErr bool
	}{
		// Valid inputs
		{"valid inputs", "main", "https://github.com/user/repo.git", false},
		{"valid git@ inputs", "feature/branch", "git@github.com:user/repo.git", false},

		// Invalid branch
		{"invalid branch", "main; rm -rf", "https://github.com/user/repo.git", true},
		{"invalid repo", "main", "https://github.com/user/repo.git; rm -rf", true},
		{"both invalid", "main; rm -rf", "https://github.com/user/repo.git; rm -rf", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitInput(tt.branch, tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitInput(%q, %q) error = %v, wantErr %v", tt.branch, tt.repo, err, tt.wantErr)
			}
		})
	}
}
