package terraform

// ResourceProvider is a legacy interface for providers.
//
// This is retained only for compatibility with legacy code. The current
// interface for providers is providers.Interface, in the sibling directory
// named "providers".
type ResourceProvider interface {
	/*********************************************************************
	* Functions related to the provider
	*********************************************************************/

	// ProviderSchema returns the config schema for the main provider
	// configuration, as would appear in a "provider" block in the
	// configuration files.
	//
	// Currently not all providers support schema. Callers must therefore
	// first call Resources and DataSources and ensure that at least one
	// resource or data source has the SchemaAvailable flag set.
	GetSchema(*ProviderSchemaRequest) (*ProviderSchema, error)

	// Input was used prior to v0.12 to ask the provider to prompt the user
	// for input to complete the configuration.
	//
	// From v0.12 onwards this method is never called because Terraform Core
	// is able to handle the necessary input logic itself based on the
	// schema returned from GetSchema.
	Input(UIInput, *ResourceConfig) (*ResourceConfig, error)

	// Validate is called once at the beginning with the raw configuration
	// (no interpolation done) and can return a list of warnings and/or
	// errors.
	//
	// This is called once with the provider configuration only. It may not
	// be called at all if no provider configuration is given.
	//
	// This should not assume that any values of the configurations are valid.
	// The primary use case of this call is to check that required keys are
	// set.
	Validate(*ResourceConfig) ([]string, []error)

	// Configure configures the provider itself with the configuration
	// given. This is useful for setting things like access keys.
	//
	// This won't be called at all if no provider configuration is given.
	//
	// Configure returns an error if it occurred.
	Configure(*ResourceConfig) error

	// Resources returns all the available resource types that this provider
	// knows how to manage.
	Resources() []ResourceType

	// Stop is called when the provider should halt any in-flight actions.
	//
	// This can be used to make a nicer Ctrl-C experience for Terraform.
	// Even if this isn't implemented to do anything (just returns nil),
	// Terraform will still cleanly stop after the currently executing
	// graph node is complete. However, this API can be used to make more
	// efficient halts.
	//
	// Stop doesn't have to and shouldn't block waiting for in-flight actions
	// to complete. It should take any action it wants and return immediately
	// acknowledging it has received the stop request. Terraform core will
	// automatically not make any further API calls to the provider soon
	// after Stop is called (technically exactly once the currently executing
	// graph nodes are complete).
	//
	// The error returned, if non-nil, is assumed to mean that signaling the
	// stop somehow failed and that the user should expect potentially waiting
	// a longer period of time.
	Stop() error

	/*********************************************************************
	* Functions related to individual resources
	*********************************************************************/

	// ValidateResource is called once at the beginning with the raw
	// configuration (no interpolation done) and can return a list of warnings
	// and/or errors.
	//
	// This is called once per resource.
	//
	// This should not assume any of the values in the resource configuration
	// are valid since it is possible they have to be interpolated still.
	// The primary use case of this call is to check that the required keys
	// are set and that the general structure is correct.
	ValidateResource(string, *ResourceConfig) ([]string, []error)

	// Apply applies a diff to a specific resource and returns the new
	// resource state along with an error.
	//
	// If the resource state given has an empty ID, then a new resource
	// is expected to be created.
	Apply(
		*InstanceInfo,
		*InstanceState,
		*InstanceDiff) (*InstanceState, error)

	// Diff diffs a resource versus a desired state and returns
	// a diff.
	Diff(
		*InstanceInfo,
		*InstanceState,
		*ResourceConfig) (*InstanceDiff, error)

	// Refresh refreshes a resource and updates all of its attributes
	// with the latest information.
	Refresh(*InstanceInfo, *InstanceState) (*InstanceState, error)

	/*********************************************************************
	* Functions related to importing
	*********************************************************************/

	// ImportState requests that the given resource be imported.
	//
	// The returned InstanceState only requires ID be set. Importing
	// will always call Refresh after the state to complete it.
	//
	// IMPORTANT: InstanceState doesn't have the resource type attached
	// to it. A type must be specified on the state via the Ephemeral
	// field on the state.
	//
	// This function can return multiple states. Normally, an import
	// will map 1:1 to a physical resource. However, some resources map
	// to multiple. For example, an AWS security group may contain many rules.
	// Each rule is represented by a separate resource in Terraform,
	// therefore multiple states are returned.
	ImportState(*InstanceInfo, string) ([]*InstanceState, error)

	/*********************************************************************
	* Functions related to data resources
	*********************************************************************/

	// ValidateDataSource is called once at the beginning with the raw
	// configuration (no interpolation done) and can return a list of warnings
	// and/or errors.
	//
	// This is called once per data source instance.
	//
	// This should not assume any of the values in the resource configuration
	// are valid since it is possible they have to be interpolated still.
	// The primary use case of this call is to check that the required keys
	// are set and that the general structure is correct.
	ValidateDataSource(string, *ResourceConfig) ([]string, []error)

	// DataSources returns all of the available data sources that this
	// provider implements.
	DataSources() []DataSource

	// ReadDataDiff produces a diff that represents the state that will
	// be produced when the given data source is read using a later call
	// to ReadDataApply.
	ReadDataDiff(*InstanceInfo, *ResourceConfig) (*InstanceDiff, error)

	// ReadDataApply initializes a data instance using the configuration
	// in a diff produced by ReadDataDiff.
	ReadDataApply(*InstanceInfo, *InstanceDiff) (*InstanceState, error)
}

// ResourceProviderCloser is an interface that providers that can close
// connections that aren't needed anymore must implement.
type ResourceProviderCloser interface {
	Close() error
}

// ResourceType is a type of resource that a resource provider can manage.
type ResourceType struct {
	Name       string // Name of the resource, example "instance" (no provider prefix)
	Importable bool   // Whether this resource supports importing

	// SchemaAvailable is set if the provider supports the ProviderSchema,
	// ResourceTypeSchema and DataSourceSchema methods. Although it is
	// included on each resource type, it's actually a provider-wide setting
	// that's smuggled here only because that avoids a breaking change to
	// the plugin protocol.
	SchemaAvailable bool
}

// DataSource is a data source that a resource provider implements.
type DataSource struct {
	Name string

	// SchemaAvailable is set if the provider supports the ProviderSchema,
	// ResourceTypeSchema and DataSourceSchema methods. Although it is
	// included on each resource type, it's actually a provider-wide setting
	// that's smuggled here only because that avoids a breaking change to
	// the plugin protocol.
	SchemaAvailable bool
}

// ResourceProviderFactory is a function type that creates a new instance
// of a resource provider.
type ResourceProviderFactory func() (ResourceProvider, error)

// ResourceProviderFactoryFixed is a helper that creates a
// ResourceProviderFactory that just returns some fixed provider.
func ResourceProviderFactoryFixed(p ResourceProvider) ResourceProviderFactory {
	return func() (ResourceProvider, error) {
		return p, nil
	}
}

func ProviderHasResource(p ResourceProvider, n string) bool {
	for _, rt := range p.Resources() {
		if rt.Name == n {
			return true
		}
	}

	return false
}

func ProviderHasDataSource(p ResourceProvider, n string) bool {
	for _, rt := range p.DataSources() {
		if rt.Name == n {
			return true
		}
	}

	return false
}

const errPluginInit = `
Plugin reinitialization required. Please run "terraform init".

Plugins are external binaries that Terraform uses to access and manipulate
resources. The configuration provided requires plugins which can't be located,
don't satisfy the version constraints, or are otherwise incompatible.

Terraform automatically discovers provider requirements from your
configuration, including providers used in child modules. To see the
requirements and constraints from each module, run "terraform providers".

%s
`
