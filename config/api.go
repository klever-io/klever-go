package config

// APIRoutesConfig holds the configuration related to Rest API routes
type APIRoutesConfig struct {
	APIPackages map[string]APIPackageConfig `yaml:"apiPackages"`
	Credentials []Credential                `yaml:"credentials"`
	Hasher      TypeConfig                  `yaml:"hasher"`
}

// APIPackageConfig holds the configuration for the routes of each package
type APIPackageConfig struct {
	Routes []RouteConfig `yaml:"routes"`
}

// RouteConfig holds the configuration for a single route
type RouteConfig struct {
	Name    string `yaml:"name"`
	Open    bool   `yaml:"open"`
	Secured bool   `yaml:"secured"`
}

// Credential holds an username and a password
type Credential struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// FacadeConfig will hold different configuration option that will be passed to the main KleverFacade
type FacadeConfig struct {
	RestAPIInterface   string `yaml:"restAPIInterface"`
	PprofEnabled       bool   `yaml:"pprofEnabled"`
	WSConnectionURL    string `yaml:"WSConnectionURL"`
	WSConnectionAPIKey string `yaml:"WSConnectionAPIKey"`
}
