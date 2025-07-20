#! /bin/bash

helm repo add minio-operator https://operator.min.io
helm install \
  --namespace minio-operator \
  --create-namespace \
  operator minio-operator/operator

kubectl get all -n minio-operator


helm install \
--namespace tenant \
--create-namespace \
--values values.yaml \
tenant minio-operator/tenant

kubectl get secret myminio-credentials -n tenant -o jsonpath="{.data.accesskey}" | base64 --decode; echo
kubectl get secret myminio-credentials -n tenant -o jsonpath="{.data.secretkey}" | base64 --decode; echo
