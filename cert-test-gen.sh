#!/bin/bash
set -e

openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 3650

CONFIG="
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
"

openssl req -x509 -config <(echo "$CONFIG") -key key.pem -out cert2.pem -days 3650 -subj "/CN=Hello/OU=TEST"
cat cert2.pem >> cert.pem
openssl pkcs12 -export -out certificate.pfx -inkey key.pem -in cert.pem
