package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the structure of the configuration file.
type Config struct {
	MySQL struct {
		Address  string `yaml:"address"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"mysql"`

	Paths struct {
		PayOverrideSource    string `yaml:"pay_override_source"`
		PayOverrideTarget    string `yaml:"pay_override_target"`
		PluginOverrideSource string `yaml:"plugin_override_source"`
		PluginOverrideTarget string `yaml:"plugin_override_target"`
	} `yaml:"paths"`

	Protection struct {
		AdminTable   bool `yaml:"admin_table"`
		PaymentTable bool `yaml:"payment_table"`
	} `yaml:"protection"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MySQL: struct {
			Address  string `yaml:"address"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		}{
			Address:  "127.0.0.1:3306",
			Username: "root",
			Password: "password",
		},
		Paths: struct {
			PayOverrideSource    string `yaml:"pay_override_source"`
			PayOverrideTarget    string `yaml:"pay_override_target"`
			PluginOverrideSource string `yaml:"plugin_override_source"`
			PluginOverrideTarget string `yaml:"plugin_override_target"`
		}{
			PayOverrideSource:    "",
			PayOverrideTarget:    "",
			PluginOverrideSource: "",
			PluginOverrideTarget: "",
		},
		Protection: struct {
			AdminTable   bool `yaml:"admin_table"`
			PaymentTable bool `yaml:"payment_table"`
		}{
			AdminTable:   true,
			PaymentTable: true,
		},
	}
}

// SaveDefaultConfig writes the default configuration to a YAML file.
func SaveDefaultConfig(filePath string) error {
	defaultCfg := DefaultConfig()
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create default config file: %w", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(defaultCfg); err != nil {
		return fmt.Errorf("failed to write default config file: %w", err)
	}

	return nil
}

// LoadConfig loads the configuration from a YAML file.
// If the file does not exist, it creates one with default values.
func LoadConfig(filePath string) (*Config, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File does not exist, create a default config
		if err := SaveDefaultConfig(filePath); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		fmt.Printf("Default config file created at %s\n", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
