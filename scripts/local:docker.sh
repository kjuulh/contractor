#!/usr/bin/env bash

set -e

cargo run -p ci -- local docker-image --image kasperhermansen/cuddle-please --tag dev --bin-name cuddle-please
