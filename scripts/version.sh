#!/bin/bash

TAG=$1

# Tag with path-relative names that match module import path
git tag $TAG
git tag drivers/aws/$TAG
git tag drivers/minio/$TAG
git tag drivers/filesystem/$TAG

# Push the correct tags
git push origin $TAG
git push origin drivers/aws/$TAG
git push origin drivers/minio/$TAG
git push origin drivers/filesystem/$TAG
