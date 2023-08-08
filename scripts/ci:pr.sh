#!/usr/bin/env bash

set -e

CMD_PREFIX="cargo run -p ci --"

CMD_PREFIX=""

if [[ -n "$CI_PREFIX" ]]; then
  CMD_PREFIX="$CI_PREFIX"
else 
  cd ci || return 1
  cargo build
  cd - || return 1
  CMD_PREFIX="ci/target/debug/ci"
fi


$CMD_PREFIX pull-request \
  --docker-image "$DOCKER_IMAGE" \
  --golang-builder-image "$GOLANG_BUILDER_IMAGE" \
  --production-image "$PRODUCTION_IMAGE" \
  --image "$REGISTRY/$SERVICE" \
  --tag "main-$(date +%s)" \
  --bin-name "$SERVICE"  