package interfaces

import "os"

// FileSystem defines operations for file system access
type FileSystem interface {
	// ReadFile reads a file and returns its contents
	ReadFile(filename string) ([]byte, error)
	// WriteFile writes data to a file
	WriteFile(filename string, data []byte, perm os.FileMode) error
	// Stat returns file info
	Stat(name string) (os.FileInfo, error)
	// MkdirAll creates a directory and all parent directories
	MkdirAll(path string, perm os.FileMode) error
	// RemoveAll removes a path and all its children
	RemoveAll(path string) error
	// Open opens a file for reading
	Open(name string) (*os.File, error)
	// Create creates a file for writing
	Create(name string) (*os.File, error)
	// OpenFile opens a file with the specified flags and permissions
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
}

// FileReader defines operations for reading files
type FileReader interface {
	ReadFile(filename string) ([]byte, error)
	Open(name string) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
}

// FileWriter defines operations for writing files
type FileWriter interface {
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Create(name string) (*os.File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
}

// DirManager defines operations for directory management
type DirManager interface {
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	Stat(name string) (os.FileInfo, error)
}
