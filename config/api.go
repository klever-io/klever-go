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

// IsRouteEnabled reports whether the named route in the given API package exists and is open.
func (c APIRoutesConfig) IsRouteEnabled(pkg, name string) bool {
	return c.routeHasFlag(pkg, name, func(r RouteConfig) bool { return r.Open })
}

// IsRouteSecured reports whether the named route in the given API package exists and is secured.
func (c APIRoutesConfig) IsRouteSecured(pkg, name string) bool {
	return c.routeHasFlag(pkg, name, func(r RouteConfig) bool { return r.Secured })
}

func (c APIRoutesConfig) routeHasFlag(pkg, name string, hasFlag func(RouteConfig) bool) bool {
	pkgConfig, ok := c.APIPackages[pkg]
	if !ok {
		return false
	}

	for _, route := range pkgConfig.Routes {
		if route.Name == name && hasFlag(route) {
			return true
		}
	}

	return false
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
