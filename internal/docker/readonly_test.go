package docker

import (
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/git"
)

// Test that readonly services are detected correctly
func TestReadonlyServiceDetection(t *testing.T) {
	readonlySvc := models.Service{
		Source: models.SourceConfig{
			Kind:   "git",
			Access: "readonly",
		},
	}

	if !git.IsReadonly(readonlySvc.Source) {
		t.Error("Expected readonly service to be detected as readonly")
	}

	editableSvc := models.Service{
		Source: models.SourceConfig{
			Kind:   "git",
			Access: "editable",
		},
	}

	if git.IsReadonly(editableSvc.Source) {
		t.Error("Expected editable service to not be detected as readonly")
	}
}
