package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types/registry"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

type RegistryAuth struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
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
		log.Printf("No auths found in config, loading default auth")
		config.Auths = loadDefaultAuth()
	} else {
		log.Printf("Found auths in config: %+v", config.Auths)
		auths := make(map[string]RegistryAuth)
		for i, auth := range config.Auths {
			if auth.Auth == "" {
				authConfig := registry.AuthConfig{
					Username: auth.Username,
					Password: auth.Password,
				}
				authStr, e := registry.EncodeAuthConfig(authConfig)
				if e != nil {
					log.Printf("Failed to encode auth for %s: %v", auth.Username, e)
					continue
				}
				auths[i] = RegistryAuth{
					Auth: authStr,
				}
				log.Printf("Encoded auth for %s", auth.Auth)
			}
		}
		config.Auths = auths
	}

	return config, nil
}

func loadDefaultAuth() map[string]RegistryAuth {

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	conf := path.Join(home, ".docker", "config.json")
	log.Printf("Looking for auth in %s", conf)
	if _, e := os.Stat(conf); e != nil {
		log.Printf("No auth found in %s", conf)
		return nil
	}

	data, err := os.ReadFile(conf)
	if err != nil {
		log.Printf("Failed to read %s: %v", conf, err)
		return nil
	}

	var dockerConfig DockerConfig
	if e := json.Unmarshal(data, &dockerConfig); e != nil {
		log.Printf("Failed to parse %s: %v", conf, e)
		return nil
	}
	auths := make(map[string]RegistryAuth)
	for i, auth := range dockerConfig.Auths {
		decodedAuth, e := base64.URLEncoding.DecodeString(auth.Auth)
		if e != nil {
			continue
		}
		credentials := strings.SplitN(string(decodedAuth), ":", 2)
		authConfig := registry.AuthConfig{
			Username: credentials[0],
			Password: credentials[1],
		}
		authStr, e := registry.EncodeAuthConfig(authConfig)
		if e != nil {
			continue
		}
		auths[i] = RegistryAuth{
			Auth: authStr,
		}
	}
	return auths
}
