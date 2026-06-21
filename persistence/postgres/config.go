package postgres

import "fmt"

// Identity provides a single reference point to be used
// as an identifier for package resources.
const Identity string = "postgres"

const (
	namedParamPrefix  string = "?"
	namedParamDivider string = "#"
)

var namedParamRegex string = fmt.Sprintf(`\%s[a-z0-9_]+(?:%s[a-z0-9]{3})?`, namedParamPrefix, namedParamDivider)

// Config contains configuration parameters for Postgres.
type Config struct {
	Host     string `yaml:"host" mapstructure:"host"`
	Port     int    `yaml:"port" mapstructure:"port"`
	Database string `yaml:"database" mapstructure:"database"`
	User     string `yaml:"user" mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"`
	PoolSize int    `yaml:"poolsize" mapstructure:"poolsize"`
	Check    bool   `yaml:"check" mapstructure:"check"`
}
