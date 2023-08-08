#!/usr/bin/env bash

set -e

docker run --rm -it -p 8000:8000 -v ${PWD}:/docs ${MKDOCS_IMAGE} 
