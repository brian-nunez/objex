#!/bin/bash

TAG=$1

if [ -z "$TAG" ]; then
  echo "Usage: $0 <tag>"
  exit 1
fi

# Tag with path-relative names that match module import path
git tag $TAG
git tag drivers/aws/$TAG
git tag drivers/filesystem/$TAG
git tag drivers/minio/$TAG

# Push the correct tags
git push origin $TAG
git push origin drivers/aws/$TAG
git push origin drivers/filesystem/$TAG
git push origin drivers/minio/$TAG
