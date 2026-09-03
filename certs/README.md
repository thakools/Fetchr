# Extra CA certificates

Drop PEM-encoded root CAs here as `*.crt` if your network intercepts TLS (Zscaler,
Netskope, corporate MITM proxies). The Docker build appends everything in this
directory to the system trust store so `npm ci` and `go mod download` can reach
their registries.

On macOS, `make certs` extracts the certificates your admin installed:

```sh
make certs
```

Leave the directory empty on networks without interception — the build works either way.

`*.crt` is gitignored so organisation-internal certificates are not committed.
