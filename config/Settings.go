package config

type Settings struct {
	// Directory in which the configuration file is located in.
	Dir string

	// Prefix used for the environment variable.
	// This will prevent accidental use of environment variables that were
	// not intended to be used for the service.
	// ex: `SERVICENAME_USER` make more sense than using `USER` because it
	// prevents you from using a system variable that you have not defined yourself.
	Prefix string

	// Default values to be used in case they are not provided
	//
	Defaults map[string]any
}
