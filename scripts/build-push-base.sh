#!/bin/sh
# This script represents how checkpoint base image was built.
newcontainer=$(buildah from scratch)
buildah config --annotation=io.kubernetes.cri-o.annotations.checkpoint.name=checkpointer $newcontainer
buildah config --annotation=org.criu.checkpoint.container.name=checkpointer $newcontainer
buildah commit $newcontainer pbaran555/checkpoint-base:1.0.0
buildah push pbaran555/checkpoint-base:1.0.0
buildah rm $newcontainer