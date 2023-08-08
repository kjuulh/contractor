#!/usr/bin/env bash

set -e

cargo run -p ci -- local build-docs --mkdocs-image $MKDOCS_IMAGE --caddy-image $CADDY_IMAGE
