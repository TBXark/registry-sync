# registry-sync
Sync images between docker registries.

## Usage

### CLI

```sh
./registry-sync -config config.json
```

### Docker

```sh
docker run -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd)/config.json:/config.json ghcr.io/tbxark/registry-sync:latest
docker run -v /var/run/docker.sock:/var/run/docker.sock --name registry-sync ghcr.io/tbxark/registry-sync:latest -config https://remote/config.json
```

## Configuration
```json
{
    "images": [
        {
            "source": "source-registry.com/image:tag",
            "target": "target-registry.com/image:tag"
        }
    ],
    "duration": 3600,
    "auths": {
        "ghcr.io": {
          "username": "user",
          "password": "password"
        },
        "docker.io": {
          "auth": "base64({\"username\":\"user\",\"password\":\"password\"})"
        }
    }
}

```

## License
**registry-sync** is licensed under the MIT License. See the [LICENSE](./LICENSE) file for more details.