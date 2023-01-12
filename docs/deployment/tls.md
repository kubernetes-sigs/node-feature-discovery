---
title: "TLS authentication"
layout: default
sort: 5
---

# Communication security with TLS
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---

NFD supports mutual TLS authentication between the nfd-master and nfd-worker
instances.  That is, nfd-worker and nfd-master both verify that the other end
presents a valid certificate.

TLS authentication is enabled by specifying `-ca-file`, `-key-file` and
`-cert-file` args, on both the nfd-master and nfd-worker instances.  The
template specs provided with NFD contain (commented out) example configuration
for enabling TLS authentication.

The Common Name (CN) of the nfd-master certificate must match the DNS name of
the nfd-master Service of the cluster. By default, nfd-master only check that
the nfd-worker has been signed by the specified root certificate (-ca-file).

Additional hardening can be enabled by specifying `-verify-node-name` in
nfd-master args, in which case nfd-master verifies that the NodeName presented
by nfd-worker matches the Common Name (CN) or a Subject Alternative Name (SAN)
of its certificate.  Note that `-verify-node-name` complicates certificate
management and is not yet supported in the helm or kustomize deployment
methods.

## Automated TLS certificate management using cert-manager

[cert-manager](https://cert-manager.io/) can be used to automate certificate
management between nfd-master and the nfd-worker pods.

The NFD source code repository contains an example kustomize overlay and helm
chart that can be used to deploy NFD with cert-manager supplied certificates
enabled.

To install `cert-manager` itself can be done as easily as this, below, or you
can refer to their documentation for other installation methods such as the
helm chart they provide.

```bash
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.6.1/cert-manager.yaml
```

To use the kustomize overlay to install node-feature-discovery with TLS enabled,
you may use the following:

```bash
kubectl apply -k deployment/overlays/samples/cert-manager
```

To make use of the helm chart, override `values.yaml` to enable both the
`tls.enabled` and `tls.certManager` options. Note that if you do not enable
`tls.certManager`, helm will successfully install the application, but
deployment will wait until certificates are manually created, as demonstrated
below.

See the sample installation commands in the Helm [Deployment](helm.md#deployment)
and [Configuration](helm.md#configuration) sections above for how to either override
individual values, or provide a yaml file with which to override default
values.

## Manual TLS certificate management

If you do not with to make use of cert-manager, the certificates can be
manually created and stored as secrets within the NFD namespace.

Create a CA certificate

```bash
openssl req -x509 -newkey rsa:4096 -keyout ca.key -nodes \
        -subj "/CN=nfd-ca" -days 10000 -out ca.crt
```

Create a common openssl config file.

```bash
cat <<EOF > nfd-common.conf
[ req ]
default_bits = 4096
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[ dn ]
C = XX
ST = some-state
L = some-city
O = some-company
OU = node-feature-discovery

[ req_ext ]
subjectAltName = @alt_names

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=@alt_names
EOF
```

Now, create the nfd-master certificate.

```bash
cat <<EOF > nfd-master.conf
.include nfd-common.conf

[ dn ]
CN = nfd-master

[ alt_names ]
DNS.1 = nfd-master
DNS.2 = nfd-master.node-feature-discovery.svc.cluster.local
DNS.3 = localhost
EOF

openssl req -new -newkey rsa:4096 -keyout nfd-master.key -nodes -out nfd-master.csr -config nfd-master.conf
```

Create certificates for nfd-worker and nfd-topology-updater

```bash
cat <<EOF > nfd-worker.conf
.include nfd-common.conf

[ dn ]
CN = nfd-worker

[ alt_names ]
DNS.1 = nfd-worker
DNS.2 = nfd-worker.node-feature-discovery.svc.cluster.local
EOF

# Config for topology updater is identical except for the DN and alt_names
sed -e 's/worker/topology-updater/g' < nfd-worker.conf > nfd-topology-updater.conf

openssl req -new -newkey rsa:4096 -keyout nfd-worker.key -nodes -out nfd-worker.csr -config nfd-worker.conf
openssl req -new -newkey rsa:4096 -keyout nfd-topology-updater.key -nodes -out nfd-topology-updater.csr -config nfd-topology-updater.conf
```

Now, sign the certificates with the CA created earlier.

```bash
for cert in nfd-master nfd-worker nfd-topology-updater; do
  echo signing $cert
  openssl x509 -req -in $cert.csr -CA ca.crt -CAkey ca.key \
    -CAcreateserial -out $cert.crt -days 10000 \
    -extensions v3_ext -extfile $cert.conf
done
```

Finally, turn these certificates into secrets.

```bash
for cert in nfd-master nfd-worker nfd-topology-updater; do
  echo creating secret for $cert in node-feature-discovery namespace
  cat <<EOF | kubectl create -n node-feature-discovery -f -
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: ${cert}-cert
data:
  ca.crt: $( cat ca.crt | base64 -w 0 )
  tls.crt: $( cat $cert.crt | base64 -w 0 )
  tls.key: $( cat $cert.key | base64 -w 0 )
EOF

done
```
