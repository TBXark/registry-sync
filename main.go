package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type RegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Images []struct {
		Source string `json:"source"`
		Target string `json:"target"`
	} `json:"images"`
	Duration int `json:"duration"`
	Auth     struct {
		Pull RegistryAuth `json:"pull"`
		Push RegistryAuth `json:"push"`
	} `json:"auth"`
}

func loadConfig(path string) (*Config, error) {
	if strings.HasPrefix(path, "http") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		config := &Config{}
		err = json.NewDecoder(resp.Body).Decode(config)
		if err != nil {
			return nil, err
		}
		return config, nil
	} else {
		bytes, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		config := &Config{}
		err = json.Unmarshal(bytes, config)
		if err != nil {
			return nil, err
		}
		return config, nil
	}
}

func main() {

	cfg := flag.String("config", "config.json", "config file")
	flag.Parse()

	config, err := loadConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
		client.WithHost("unix:///var/run/docker.sock"),
	)
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	pullAuth, err := json.Marshal(config.Auth.Pull)
	if err != nil {
		log.Fatal("Failed to marshal pull auth:", err)
	}
	pushAuth, err := json.Marshal(config.Auth.Push)
	if err != nil {
		log.Fatal("Failed to marshal push auth:", err)
	}

	pull := image.PullOptions{
		RegistryAuth: base64.StdEncoding.EncodeToString(pullAuth),
		All:          true,
	}
	push := image.PushOptions{
		RegistryAuth: base64.StdEncoding.EncodeToString(pushAuth),
		All:          true,
	}

	readAllToDiscard := func(r io.ReadCloser) error {
		defer r.Close()
		_, e := io.Copy(io.Discard, r)
		return e
	}

	for {
		for _, img := range config.Images {
			log.Printf("start to process image %s", img.Source)

			// Pull image
			reader, e := cli.ImagePull(context.Background(), img.Source, pull)
			if e != nil {
				log.Printf("pull image %s failed: %v", img.Source, e)
				continue
			}
			if re := readAllToDiscard(reader); re != nil {
				log.Printf("error while pulling image %s: %v", img.Source, e)
				continue
			}
			log.Printf("pull image %s success", img.Source)

			// Tag image
			if e = cli.ImageTag(context.Background(), img.Source, img.Target); e != nil {
				log.Printf("tag image %s to %s failed: %v", img.Source, img.Target, e)
				continue
			}
			log.Printf("tag image %s to %s success", img.Source, img.Target)

			// Push image
			reader, e = cli.ImagePush(context.Background(), img.Target, push)
			if e != nil {
				log.Printf("push image %s failed: %v", img.Target, e)
				continue
			}
			if re := readAllToDiscard(reader); re != nil {
				log.Printf("error while pushing image %s: %v", img.Target, e)
				continue
			}
			log.Printf("push image %s success", img.Target)
		}
		log.Printf("sleep %d seconds", config.Duration)
		time.Sleep(time.Duration(config.Duration) * time.Second)
		conf, e := loadConfig(*cfg)
		if e != nil {
			log.Printf("reload config failed: %v", e)
		}
		config = conf
	}
}
