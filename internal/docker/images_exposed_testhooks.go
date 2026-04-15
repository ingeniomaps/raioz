package docker

// SetExposedPortCacheForTest seeds the ExposedPorts cache with a known
// value. Exported for cross-package tests (upcase/enrich) so they can
// drive GetImageExposedPort without shelling out to docker. Not intended
// for production callers.
func SetExposedPortCacheForTest(image string, port int, err error) {
	exposedPortCache.Store(image, exposedPortEntry{port: port, err: err})
}
