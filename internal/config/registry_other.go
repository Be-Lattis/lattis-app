//go:build !windows

package config

func registrySources() []Source       { return nil }
func registryPolicySources() []Source { return nil }
