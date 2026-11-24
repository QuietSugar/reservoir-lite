# SSL Certificate Generation

To enable HTTPS MITM caching, generate your own CA certificate and private key in this folder:

```bash
openssl genrsa -out ssl/ca.key 2048
openssl req -x509 -new -nodes -key ssl/ca.key -sha256 -days 3650 -out ssl/ca.crt -subj "//CN=reservoir"
```

> **Security Notice**: Never commit `ca.key` or production certificates to version control.
