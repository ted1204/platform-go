#! /bin/bash

curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube
minikube version

minikube start \
  --driver=docker \
  --container-runtime=docker \
  --cni=bridge \
  --kubernetes-version=stable \
  --extra-config=kubelet.cgroup-driver=systemd \
  --wait=true \
  --wait-timeout=5m \
  --memory=4g \
  --cpus=2

