#!/usr/bin/env bash

set -e

docker run --rm -it -v ${PWD}:/docs ${MKDOCS_IMAGE} new .
