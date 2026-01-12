package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	DefaultModel string `mapstructure:"default_model"`
	OutputDir    string `mapstructure:"output_dir"`
	Timestamps   bool   `mapstructure:"timestamps"`
}

// TranscriptionConfig holds settings for a single transcription job.
type TranscriptionConfig struct {
	URL        string
	Model      string
	Timestamps bool
	OutputDir  string
}

// Load reads configuration from file and environment.
func Load(cfgFile string) (*Config, error) {
	cfg := &Config{
		DefaultModel: "base",
		OutputDir:    getDefaultOutputDir(),
		Timestamps:   false,
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".config", "whisper-transcribe"))
		}
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("WHISPER")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getDefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./transcripts"
	}
	return filepath.Join(home, "transcripts")
}

// ModelOptions returns available Whisper model options.
func ModelOptions() []string {
	return []string{"tiny", "base", "small", "medium", "large"}
}
