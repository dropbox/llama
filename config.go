package llama

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"net"
)

// A sensible default configuration for the collector in YAML
var defaultCollectorConfigYAML = `
summarization:
    interval:   30
    handlers:   2

api:
    bind:   0.0.0.0:5000

ports:
    default:
        ip:         0.0.0.0
        port:       0
        tos:        0
        timeout:    1000
port_groups:
    default:
        - port:     default
          count:    4
rate_limits:
    default:
        cps:    4.0
tests:
    - targets:      default
      port_group:   default
      rate_limit:   default
targets:
    default:
        - ip:       127.0.0.1
          port:     8100
          tags:     {}
`

// PortConfig describes the configuration for a single Port.
type PortConfig struct {
	IP      string `yaml:"ip"`
	Port    int64  `yaml:"port"`
	Tos     int64  `yaml:"tos"`
	Timeout int64  `yaml:"timeout"`
}

// PortsConfig is a mapping of port "name" to a PortConfig.
type PortsConfig map[string]PortConfig

// PortGroupConfig describes a set of identical Ports in a PortGroup.
type PortGroupConfig struct {
	Port  string `yaml:"port"` // Should correspond with a PortsConfig key
	Count int64  `yaml:"count"`
}

// PortGroupsConfig is a mapping of port group "name" to PortGroupConfigs.
type PortGroupsConfig map[string][]PortGroupConfig

// RateLimitConfig describes the configuration for a rate limiter.
type RateLimitConfig struct {
	CPS float64 `yaml:"cps"` // Cycles per second
}

// RateLimitsConfig is a mapping of "name" to RateLimitConfig.
type RateLimitsConfig map[string]RateLimitConfig

// TestConfig describes the elements of a test, for use by TestRunner, which
// correspond to their respective named elements in the config.
//
// Ex. A `targets` value of "default" in the config would correspond to a
// TargetsConfig key of "default" which contains the definitions of targets.
type TestConfig struct {
	Targets   string `yaml:"targets"`    // Should correspond with a TargetsConfig key
	PortGroup string `yaml:"port_group"` // Should correspond with a PortGroupsConfig key
	RateLimit string `yaml:"rate_limit"` // Should correspond with a RateLimitsConfig key
}

// TestsConfig is a slice of TestConfig structs.
type TestsConfig []TestConfig

// TargetConfig describes a single target for testing, including tags that
// are applied to the resulting summaries.
//
// TODO(dmar): Restructure this to be more Dropbox specific, and reduce the
//      data being included in this config. Most of this can come from a base,
//      and then be populated by MDB queries.
type TargetConfig struct {
	IP   string `yaml:"ip"`
	Port int64  `yaml:"port"`
	Tags Tags   `yaml:"tags"`
}

// AddrString converts the tc into a string formated "IP:port" combo.
func (tc *TargetConfig) AddrString() string {
	str := fmt.Sprintf("%v:%v", tc.IP, tc.Port)
	return str
}

// ResolveUDPAddr converts the tc into a net.UDPAddr pointer.
func (tc *TargetConfig) ResolveUDPAddr() (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", tc.AddrString())
}

// TargetSet is a slice of TargetConfig structs.
type TargetSet []TargetConfig

// TagSet converts the ts into TagSet struct.
func (ts TargetSet) TagSet() TagSet {
	tagset := make(TagSet)
	ts.IntoTagSet(tagset)
	return tagset
}

// IntoTagSet is similar to TagSet but updates the provided tagset instead of
// creating a new one.
func (ts TargetSet) IntoTagSet(tagset TagSet) {
	for _, target := range ts {
		key := target.IP
		// If the IP/key already exists, this will override it
		tagset[key] = target.Tags
	}
}

// ListTargets provides a slice of "IP:port" string representations for all of
// the targets in the ts.
func (ts TargetSet) ListTargets() []string {
	addrs := make([]string, 0)
	for _, target := range ts {
		addrs = append(addrs, target.AddrString())
	}
	return addrs
}

// ListResolvedTargets provides a slice of net.UDPAddr pointers for all of
// the targets in the ts, and will return with an error as soon as one is hit.
func (ts TargetSet) ListResolvedTargets() ([]*net.UDPAddr, error) {
	addrs := make([]*net.UDPAddr, 0)
	for _, target := range ts {
		addr, err := target.ResolveUDPAddr()
		if err != nil {
			return addrs, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// TargetsConfig is a mapping of "name" to TargetSet slice.
type TargetsConfig map[string]TargetSet

// TagSet is a wrapper, and merges the TagSet output for all TargetSet slices
// within the tc.
func (tc TargetsConfig) TagSet() TagSet {
	ts := make(TagSet)
	tc.IntoTagSet(ts)
	return ts
}

// IntoTagSet is a wrapper about the same function for each contained TargetSet
// and merges them into an existing ts.
func (tc TargetsConfig) IntoTagSet(ts TagSet) {
	// TODO(dmar): Right now, this doesn't distinguish by TargetSet, so if a
	//      target appears in multiple places, only the last entry will be
	//      used.
	for _, targetSet := range tc {
		targetSet.IntoTagSet(ts)
	}
}

// SummarizationConfig describes the parameters for setting up a Summarizer
// and related ResultHandlers.
type SummarizationConfig struct {
	Interval int64 `yaml:"interval"`
	Handlers int64 `yaml:"handlers"`
}

// APIConfig describes the parameters for the JSON HTTP API.
type APIConfig struct {
	Bind string `yaml:"bind"`
}

// CollectorConfig wraps all of the above structs/maps/slices and defines the
// overall configuration for a collector.
type CollectorConfig struct {
	Summarization SummarizationConfig `yaml:"summarization"`
	API           APIConfig           `yaml:"api"`
	Ports         PortsConfig         `yaml:"ports"`
	PortGroups    PortGroupsConfig    `yaml:"port_groups"`
	RateLimits    RateLimitsConfig    `yaml:"rate_limits"`
	Tests         TestsConfig         `yaml:"tests"`
	Targets       TargetsConfig       `yaml:"targets"`
}

//
// Config Creators
//

// NewDefaultCollectorConfig provides a sensible default collector config.
func NewDefaultCollectorConfig() (*CollectorConfig, error) {
	return NewCollectorConfig([]byte(defaultCollectorConfigYAML))
}

// NewCollectorConfig provides a parsed CollectorConfig based on the provided
// data.
//
// `data` is expected to be a byte slice version of a YAML CollectorConfig.
func NewCollectorConfig(data []byte) (*CollectorConfig, error) {
	cc := &CollectorConfig{}
	err := yaml.Unmarshal(data, cc)
	if err != nil {
		return cc, fmt.Errorf("Failed to parse collector config: %s", err)
	}
	return cc, nil
}

// LegacyCollectorConfig is for backward compatibility with the existing LLAMA
// config and represents only a map of targets to tags.
type LegacyCollectorConfig map[string]map[string]string

// ToDefaultCollectorConfig converts a LegacyCollectorConfig to CollectorConfig
// by merging with the defaults and applying the provided port on targets.
func (legacy *LegacyCollectorConfig) ToDefaultCollectorConfig(port int64) (*CollectorConfig, error) {
	cc, err := NewDefaultCollectorConfig()
	if err != nil {
		return cc, err
	}
	// Override the default targets
	cc.Targets = make(TargetsConfig)
	// Parse the targets from the old style into the new one.
	// This requires the "default" options from above for now.
	for addr, tags := range *legacy {
		cc.Targets["default"] = append(cc.Targets["default"], TargetConfig{
			IP:   addr,
			Port: port,
			Tags: tags,
		})
	}
	return cc, nil
}

// NewLegacyCollectorConfig creates a new LegacyCollectorConfig struct based
// on the provided data, which is expected to be a YAML representation of the
// config.
func NewLegacyCollectorConfig(data []byte) (*LegacyCollectorConfig, error) {
	lcc := make(LegacyCollectorConfig)
	err := yaml.Unmarshal(data, lcc)
	if err != nil {
		return &lcc, fmt.Errorf("Failed to parse legacy collector config: %s", err)
	}
	return &lcc, nil
}
