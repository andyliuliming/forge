package forge

import (
	"io"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/sclevine/forge/engine"
)

//go:generate mockgen -package mocks -destination mocks/versioner.go github.com/sclevine/forge Versioner
type Versioner interface {
	Build(tmplURL, versionURL string) (string, error)
}

//go:generate mockgen -package mocks -destination mocks/image.go github.com/sclevine/forge Image
type Image interface {
	Pull(image string) <-chan engine.Progress
	Build(tag string, dockerfile engine.Stream) <-chan engine.Progress
}

//go:generate mockgen -package mocks -destination mocks/container.go github.com/sclevine/forge Container
type Container interface {
	io.Closer
	ID() string
	CloseAfterStream(stream *engine.Stream) error
	Background() error
	Start(logPrefix string, logs io.Writer, restart <-chan time.Time) (status int64, err error)
	HealthCheck() <-chan string
	Commit(ref string) (imageID string, err error)
	ExtractTo(tar io.Reader, path string) error
	CopyTo(stream engine.Stream, path string) error
	CopyFrom(path string) (engine.Stream, error)
}

//go:generate mockgen -package mocks -destination mocks/engine.go github.com/sclevine/forge Engine
type Engine interface {
	NewContainer(config *container.Config, hostConfig *container.HostConfig) (Container, error)
}

type Loader interface {
	Loading(message string, progress <-chan engine.Progress) error
}

type Colorizer func(string, ...interface{}) string

type vcapApplication struct {
	ApplicationID      string           `json:"application_id"`
	ApplicationName    string           `json:"application_name"`
	ApplicationURIs    []string         `json:"application_uris"`
	ApplicationVersion string           `json:"application_version"`
	Host               string           `json:"host,omitempty"`
	InstanceID         string           `json:"instance_id,omitempty"`
	InstanceIndex      *uint            `json:"instance_index,omitempty"`
	Limits             map[string]int64 `json:"limits"`
	Name               string           `json:"name"`
	Port               *uint            `json:"port,omitempty"`
	SpaceID            string           `json:"space_id"`
	SpaceName          string           `json:"space_name"`
	URIs               []string         `json:"uris"`
	Version            string           `json:"version"`
}
