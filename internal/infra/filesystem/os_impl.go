package filesystem

import (
	"os"

	"raioz/internal/domain/interfaces"
)

// Ensure OSFileSystem implements interfaces.FileSystem
var _ interfaces.FileSystem = (*OSFileSystem)(nil)

// OSFileSystem is the concrete implementation of FileSystem using os package
type OSFileSystem struct{}

// NewOSFileSystem creates a new OSFileSystem implementation
func NewOSFileSystem() interfaces.FileSystem {
	return &OSFileSystem{}
}

// ReadFile reads a file and returns its contents
func (fs *OSFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile writes data to a file
func (fs *OSFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// Stat returns file info
func (fs *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// MkdirAll creates a directory and all parent directories
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveAll removes a path and all its children
func (fs *OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Open opens a file for reading
func (fs *OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// Create creates a file for writing
func (fs *OSFileSystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

// OpenFile opens a file with the specified flags and permissions
func (fs *OSFileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
