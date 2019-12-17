package chshare

import (
	"encoding/json"
	"fmt"
)

// Config describes a chisel proxy/client session configuration. It is
// sent from the client to the server during initialization
type Config struct {
	Version string
	ChannelDescriptors []*ChannelDescriptor
}

// DecodeConfig unserializes a chisel session Config from JSON
func DecodeConfig(b []byte) (*Config, error) {
	c := &Config{}
	err := json.Unmarshal(b, c)
	if err != nil {
		return nil, fmt.Errorf("Invalid JSON config")
	}
	return c, nil
}

// EncodeConfig serializes a chisel session Config from JSON
func EncodeConfig(c *Config) ([]byte, error) {
	return json.Marshal(c)
}
