package viper

import (
	"strings"

	"github.com/spf13/viper"
)

// New returns a Viper instance that reads from environment variables and,
// if envFile is non-empty, from that file (e.g. ".env"). Missing files are
// silently ignored so the app works without a .env in production.
func New(envFile string) *viper.Viper {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if envFile != "" {
		v.SetConfigFile(envFile)
		_ = v.ReadInConfig()
	}
	return v
}
