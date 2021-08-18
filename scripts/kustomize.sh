#!/bin/bash -e

echo "namespace: $1"
echo "image: $2:$3"

cat > kustomization.yaml << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: $1

images:
- name: '*'
  newName: $2
  newTag: $3

resources:
- deployment/overlays/default
EOF
