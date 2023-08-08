# Contractor - A renovate bot for gitea and github

Contractor is a chatops like bot, integrating with github/gitea issues, allowing
commands to trigger renovate runs.

```bash
/contractor refresh
```

Contractor runs in a regular docker image and uses the official renovate slim
image behind the scenes, this can be changed in the configuration.

![command](./assets/command.png)
<small>Do note that the contractor was run under a personal user, hence the same
user replied</small>

## Motivation

Renovate by default if hosted yourself, is neither sharded, or runs on a
cron-job cycle. This leaves a lot to be desired from a developers point of view.
As it may take quite a long time for renovate to revisit the pull-request again,
if there is a lot of repositories enabled.

This project intends to add an ad-hoc invocation of renovate for a single
repository, this enables developers to retrigger renovate whenever they want.

The project is built to be integrated with github and gitea (initially), and
work in its pull-request system, so when a renovate pr shows up, you can either
manually retrigger it, or enable any of the options in the renovate dashboard,
and retrigger.

## DISCLAIMER

The project is still 0.x.x As such the api is subject to change, and the
examples will probably be out of date. The below should be seen as an example of
what the project will look like once feature-complete.

## Milestones

- [x] 0.1.0
  - Includes basic setup such as working server bot, and installation command,
    automation is missing however. Also only gitea support for now, because this
    is where the project initially is supposed to be in use.
- [ ] 0.2.0
  - Add GitHub support
- [ ] 0.3.0
  - Add Delegation support (not clustering, just delegation of renovate jobs)
- [ ] 0.4.0
  - Slack integration
- [ ] 0.5.0
  - GitHub App and such support
- [ ] 0.6.0
  - Add api key support

## Getting started

First you need to pull and run the contractor image, docker is the preferred way
of execution, but golang is also available from source.

Docker compose is given as an example, but you're free to run using `docker run`
if you prefer.

See example for a ready-to-run image

```yaml
# file: docker-compose.yaml
version: "3"
services:
  contractor:
  image: docker.io/kjuulh/contractor:latest
  restart: unless-stopped
  commands:
  - contractor server serve
  volumes:
  - "./templates/contractor:/mnt/config"
  - "/var/run/docker.sock:/var/run/docker.sock"
  env_file:
  - .env
```

```bash
# file: .env
GITEA_RENOVATE_TOKEN=<gitea application token> # needs repo and pull request permissions
GITHUB_RENOVATE_TOKEN=<github personel access token> # needs repo and pull request permissions
GITHUB_COM_TOKEN=<github personel access token> # used for communication, doesn't need much
RENOVATE_SECRETS='{"HARBOR_SERVER_PASSWORD": "<whatever secret you need in your config>"}'
CONTRACTOR_API_KEY='<some sufficiently secret password used for webhooks to authenticate to your server>'
```

```json
// file: templates/contractor/config.json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "platform": "gitea",
  "extends": [
    "config:base"
  ]
}
// Remember to fill out the options as you see fit, this is not a complete example
```

Use renovate secret for each `{{ secrets.HARBOR_SERVER_PASSWORD }}` in your
config, replace `HARBOR_SERVER_PASSWORD` with your own

And then run the server with: `docker compose up`

This has started the server, but github doesn't know that it needs to talk to
you yet.

As such host the server somewhere with a public hostname, such that github or
gitea webhooks can reach it, i.e. contractor.some-domain.com:9111

To install the webhook, either use the docker image, or download the cli from
source.

### CLI

To install the cli

```bash
go install git.front.kjuulh.io/kjuulh/contractor@latest
```

contractor will automatically read any .env file, so you can leave out the
secrets.

```bash
contractor install  \
--owner kjuulh \
--repository contractor \
--url https://git.front.kjuulh.io/api/v1 \
--backend gitea
```

If you leave any of these out, contractor will prompt your for required values.

### Docker

You can also use docker for it.

```bash
docker compose run contractor \
install \
--owner kjuulh \
--repository contractor \
--url https://git.front.kjuulh.io/api/v1 \
--backend gitea
```

### GitHub App

TBD, this should automatically install the webhook for allowed repositories, I
just haven't gotten around to it yet. It is on the 0.3.0 Roadmap.
