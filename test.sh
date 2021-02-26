#!/bin/bash

set -o errexit
set -o pipefail
set -x

K8S_VERSION=v1.19.7
START_DELAY=800ms
SCRATCH=$(mktemp -d)

cat > $SCRATCH/config <<EOF
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: worker
  image: kindest/node:${K8S_VERSION}
- role: control-plane
  image: kindest/node:${K8S_VERSION}
EOF


if [ -n "${SKIP_BUILD}" ]; then
cat > images.env<<EOF
APP_IMAGE=$(cd app; ko publish -P .)
PROBE_IMAGE=$(cd probe; ko publish -P .)
EOF
fi

source images.env

if [ -z "${SKIP_SETUP}" ]; then
  kind delete cluster --name probing-demo
  kind create cluster --name probing-demo --config "${SCRATCH}/config"

  docker pull "${APP_IMAGE}"
  docker pull "${PROBE_IMAGE}"

  kind load docker-image --name probing-demo "${APP_IMAGE}"
  kind load docker-image --name probing-demo "${PROBE_IMAGE}"

  kubectl wait node --for=condition=Ready --timeout=60s --all
fi


kind get kubeconfig --name probing-demo > "${SCRATCH}/kubeconfig"
export KUBECONFIG="${SCRATCH}/kubeconfig"


cat > "${SCRATCH}/regular-probe" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: regular-probe
  labels:
    app: regular-probe
spec:
  containers:
  - name: app
    image: ${APP_IMAGE}
    env:
    - name: START_DELAY
      value: ${START_DELAY}
    ports:
    - containerPort: 8080
    readinessProbe:
      periodSeconds: 1
      httpGet:
        path: /
        port: 8080
EOF

cat > "${SCRATCH}/aggressive-probe" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: aggressive-probe
  labels:
    app: aggressive-probe
spec:
  containers:
  - name: app
    image: ${APP_IMAGE}
    env:
    - name: START_DELAY
      value: ${START_DELAY}
    ports:
    - containerPort: 8080
  - name: probe
    image: ${PROBE_IMAGE}
    ports:
    - containerPort: 8081
    readinessProbe:
      periodSeconds: 1
      timeoutSeconds: 10
      httpGet:
        path: /
        port: 8081
EOF

kubectl delete -f "${SCRATCH}/regular-probe" --ignore-not-found=true
kubectl apply -f "${SCRATCH}/regular-probe"
time kubectl wait pod --for=condition=Ready --timeout=20s -l app=regular-probe

kubectl delete -f "${SCRATCH}/aggressive-probe" --ignore-not-found=true
kubectl apply -f "${SCRATCH}/aggressive-probe"
time kubectl wait pod --for=condition=Ready --timeout=20s -l app=aggressive-probe
