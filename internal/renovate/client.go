package renovate

import (
	"context"
	"fmt"
	"log"
	"os"

	"dagger.io/dagger"
)

type RenovateClient struct {
	config string
}

func NewRenovateClient(config string) *RenovateClient {
	return &RenovateClient{config: config}
}

func (rc *RenovateClient) RefreshRepository(ctx context.Context, owner, repository string) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}

	envRenovateToken := os.Getenv("GITEA_RENOVATE_TOKEN")
	log.Println(envRenovateToken)

	renovateToken := client.SetSecret("RENOVATE_TOKEN", envRenovateToken)
	githubComToken := client.SetSecret("GITHUB_COM_TOKEN", os.Getenv("GITHUB_COM_TOKEN"))
	renovateSecret := client.SetSecret("RENOVATE_SECRETS", os.Getenv("RENOVATE_SECRETS"))

	output, err := client.Container().
		From("renovate/renovate:latest").
		WithNewFile("/opts/renovate/config.json", dagger.ContainerWithNewFileOpts{
			Contents: `{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "platform": "gitea",
  "endpoint": "https://git.front.kjuulh.io/api/v1/",
  "automerge": true,
  "automergeType": "pr",
  "extends": [
    "config:base"
  ],
  "hostRules": [
    {
      "hostType": "docker",
      "matchHost": "harbor.front.kjuulh.io",
      "username": "service",
      "password": "{{ secrets.HARBOR_SERVER_PASSWORD }}"
    }
  ],
  "packageRules": [
    {
      "matchDatasources": ["docker"],
      "registryUrls": ["https://harbor.front.kjuulh.io/docker-proxy/library/"]
    },
    {
      "groupName": "all dependencies",
      "separateMajorMinor": false,
      "groupSlug": "all",
      "packageRules": [
        {
          "matchPackagePatterns": [
            "*"
          ],
          "groupName": "all dependencies",
          "groupSlug": "all"
        }
      ],
      "lockFileMaintenance": {
        "enabled": false
      }
    }
  ]
}`,
			Permissions: 755,
			Owner:       "root",
		}).
		WithSecretVariable("RENOVATE_TOKEN", renovateToken).
		WithSecretVariable("GITHUB_COM_TOKEN", githubComToken).
		WithSecretVariable("RENOVATE_SECRETS", renovateSecret).
		WithEnvVariable("LOG_LEVEL", "warn").
		WithEnvVariable("RENOVATE_CONFIG_FILE", "/opts/renovate/config.json").
		WithExec([]string{
			fmt.Sprintf("%s/%s", owner, repository),
		}).
		Sync(ctx)

	stdout, outerr := output.Stdout(ctx)
	if outerr == nil {
		log.Printf("stdout: %s", stdout)
	}
	stderr, outerr := output.Stderr(ctx)
	if outerr == nil {
		log.Printf("stderr: %s", stderr)
	}
	if err != nil {
		return fmt.Errorf("error: %w, \nstderr: %s\nstdout: %s", err, stderr, stdout)
	}

	return nil
}
