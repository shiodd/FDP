# FDP - File Download Proxy

A simple file download proxy service that supports downloading files by URL concatenation.

## Features

- Support HTTP/HTTPS protocol downloads
- Support range requests (resume downloads)
- Built-in SSRF protection

## Usage Example

```
http://localhost:9517/https://example.com/image.jpg
```

## Quick Start

### Local Development (Requires Go)

```bash
go run main.go
```

Default port is `9517`

### Docker

```bash
docker run -d \
  --name file-download-proxy \
  -p 9517:9517 \
  --restart unless-stopped \
  shiodd/file-download-proxy:latest
```

After starting, visit `http://localhost:9517` and you should see `success`.

### Docker Build

```bash
docker build -t myapp:latest .
```

## License

MIT
