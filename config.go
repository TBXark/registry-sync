package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

type RegistryAuth struct {
	Auth string `json:"auth"`
}

type ImageConfig struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type DockerConfig struct {
	Auths map[string]RegistryAuth `json:"auths"`
}

type Config struct {
	Images       []ImageConfig           `json:"images"`
	Auths        map[string]RegistryAuth `json:"auths"`
	Duration     int                     `json:"duration"`
	DisablePrune bool                    `json:"disable_prune"`
}

func loadConfig(path string) (*Config, error) {
	var body []byte
	var err error

	if strings.HasPrefix(path, "http") {
		resp, httpErr := http.Get(path)
		if httpErr != nil {
			return nil, fmt.Errorf("failed to fetch config: %w", httpErr)
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
	} else {
		body, err = os.ReadFile(path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	config := &Config{}
	if e := json.Unmarshal(body, config); e != nil {
		return nil, fmt.Errorf("failed to parse config: %w", e)
	}

	if config.Auths == nil || len(config.Auths) == 0 {
		config.Auths = loadDefaultAuth()
	}

	return config, nil
}

func loadDefaultAuth() map[string]RegistryAuth {

	auths := make(map[string]RegistryAuth)
	home, err := os.UserHomeDir()
	if err != nil {
		return auths
	}

	conf := path.Join(home, ".docker", "config.json")
	if _, e := os.Stat(conf); e != nil {
		return auths
	}

	data, err := os.ReadFile(conf)
	if err != nil {
		return auths
	}

	var dockerConfig DockerConfig
	if e := json.Unmarshal(data, &dockerConfig); e != nil {
		return auths
	}

	for registry, auth := range dockerConfig.Auths {
		log.Printf("load auth for registry %s", registry)
		auths[registry] = RegistryAuth{
			Auth: auth.Auth,
		}
	}
	return auths
}
