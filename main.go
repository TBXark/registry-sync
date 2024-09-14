package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

var BuildVersion = "dev"

type RegistryAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string
}

type RegistryBase64Auth struct {
	Auth string `json:"auth"`
}

type ImageConfig struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type DockerConfig struct {
	Auths map[string]RegistryBase64Auth `json:"auths"`
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

	if config.Auths != nil && len(config.Auths) > 0 {
		for _, auth := range config.Auths {
			auth.Auth = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)))
		}
	} else {
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
		auths[registry] = RegistryAuth{
			Auth: auth.Auth,
		}
	}
	return auths
}

func main() {
	cfg := flag.String("config", "config.json", "config file")
	help := flag.Bool("help", false, "show help")
	flag.Parse()

	if *help {
		fmt.Printf("Version: %s\n", BuildVersion)
		flag.Usage()
		return
	}

	config, err := loadConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	for {
		if e := processImages(cli, config); e != nil {
			log.Printf("Error processing images: %v", e)
		}

		if !config.DisablePrune {
			if e := pruneUnusedImages(cli); e != nil {
				log.Printf("Error pruning unused images: %v", e)
			}
		}

		if newConfig, e := loadConfig(*cfg); e == nil {
			config = newConfig
		}

		log.Printf("Sleeping for %d seconds", config.Duration)
		time.Sleep(time.Duration(config.Duration) * time.Second)
	}
}

func processImages(cli *client.Client, config *Config) error {
	g := new(errgroup.Group)
	for _, img := range config.Images {
		img := img
		g.Go(func() error {
			pull := image.PullOptions{
				All: true,
			}
			push := image.PushOptions{
				All: true,
			}
			if config.Auths != nil {
				for registry, auth := range config.Auths {
					if strings.HasPrefix(img.Source, registry) {
						pull.RegistryAuth = auth.Auth
					}
					if strings.HasPrefix(img.Target, registry) {
						push.RegistryAuth = auth.Auth
					}
				}
			}
			return processImage(cli, &img, &pull, &push)
		})
	}
	return g.Wait()
}

func readAllToDiscard(r io.ReadCloser) error {
	defer r.Close()
	_, e := io.Copy(io.Discard, r)
	return e
}

func processImage(cli *client.Client, img *ImageConfig, pull *image.PullOptions, push *image.PushOptions) error {
	log.Printf("start to process image %s", img.Source)

	// Pull image
	reader, e := cli.ImagePull(context.Background(), img.Source, *pull)
	if e != nil {
		return fmt.Errorf("pull image %s failed: %w", img.Source, e)
	}
	if re := readAllToDiscard(reader); re != nil {
		return fmt.Errorf("error while pulling image %s: %w", img.Source, re)
	}
	log.Printf("pull image %s success", img.Source)

	// Tag image
	if e = cli.ImageTag(context.Background(), img.Source, img.Target); e != nil {
		return fmt.Errorf("tag image %s to %s failed: %w", img.Source, img.Target, e)
	}
	log.Printf("tag image %s to %s success", img.Source, img.Target)

	// Push image
	reader, e = cli.ImagePush(context.Background(), img.Target, *push)
	if e != nil {
		return fmt.Errorf("push image %s failed: %w", img.Target, e)
	}
	if re := readAllToDiscard(reader); re != nil {
		return fmt.Errorf("error while pushing image %s: %w", img.Target, re)
	}
	log.Printf("push image %s success", img.Target)

	return nil
}

func pruneUnusedImages(cli *client.Client) error {
	log.Println("Pruning unused and untagged images")

	images, err := cli.ImageList(context.Background(), image.ListOptions{
		All: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	var spaceReclaimed int64
	var deletedCount int

	for _, img := range images {
		if len(img.RepoTags) > 0 {
			continue
		}
		if len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && strings.HasSuffix(img.RepoTags[0], ":<none>")) {
			_, e := cli.ImageRemove(context.Background(), img.ID, image.RemoveOptions{Force: true, PruneChildren: true})
			if e != nil {
				imageName := "<unnamed>"
				if len(img.RepoTags) > 0 {
					imageName = img.RepoTags[0]
				}
				log.Printf("Failed to remove image %s (ID: %s): %v", imageName, img.ID, e)
				continue
			}
			spaceReclaimed += img.Size
			deletedCount++
			log.Printf("Removed image: %s", img.ID)
		}
	}

	log.Printf("Pruned %d images, reclaimed space: %d bytes", deletedCount, spaceReclaimed)
	return nil
}
