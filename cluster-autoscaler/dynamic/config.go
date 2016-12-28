package dynamic

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/nodegroup"
	"k8s.io/kubernetes/pkg/api/v1"
)

// Config which represents not static but dynamic configuration of cluster-autoscaler which would be updated periodically at runtime
type Config struct {
	Settings
	resourceVersion string
}

// Settings of cluster-autoscaler contained in the latest config, which should be consumed by cluster-autoscaler
type Settings struct {
	NodeGroups []nodegroup.Spec `json:"nodeGroups"`
}

// NewDefaultConfig builds a new config object
func NewDefaultConfig() Config {
	return Config{
		Settings: Settings{
			NodeGroups: []nodegroup.Spec{},
		},
		resourceVersion: "",
	}
}

// ConfigFromConfigMap returns the configuration read from a configmap
func ConfigFromConfigMap(configmap *v1.ConfigMap) (*Config, error) {
	settingsInJson := configmap.Data["settings"]

	if settingsInJson == "" {
		return nil, fmt.Errorf(`invalid format of configmap: missing the key named "nodeGroups" in config = %v`, settingsInJson)
	}

	settings := Settings{}
	if err := json.Unmarshal([]byte(settingsInJson), &settings); err != nil {
		return nil, fmt.Errorf(`failed to parse configmap data: %v`, err)
	}

	config := &Config{
		Settings:        settings,
		resourceVersion: configmap.ResourceVersion,
	}

	glog.V(5).Infof("json=%v settings=%v config=%v", settingsInJson, settings, config)

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config : %v", err)
	}

	return config, nil
}

// VersionMismatchesAgainst returns true if versions between two configs don't match i.e. the config should be updated.
func (c Config) VersionMismatchesAgainst(other Config) bool {
	return c.resourceVersion != other.resourceVersion
}

// NodeGroupSpecStrings returns node group specs represented in the form of `<minSize>:<maxSize>:<name>` to be passed to cloudprovider impls.
func (c Config) NodeGroupSpecStrings() []string {
	return c.nodeGroupSpecStrings()
}

func (c Config) validate() error {
	for _, g := range c.NodeGroups {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("invalid node group: %v", err)
		}
	}
	return nil
}

func (s Settings) nodeGroupSpecStrings() []string {
	result := []string{}

	for _, spec := range s.NodeGroups {
		result = append(result, spec.String())
	}

	return result
}
