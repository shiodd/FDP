# FDP(File Download Proxy)

[中文](#中文) | [English](./README_en.md)


一个简单的文件下载代理服务，支持通过 URL 拼接下载文件。

### 功能特性

- 支持 HTTP/HTTPS 协议下载
- 支持断点续传
- 内置防 SSRF 保护

### 使用示例

```
http://localhost:9517/https://example.com/image.jpg
```

### 快速开始

#### 本地运行（需要 Go 环境）

```bash
go run main.go
```

默认端口为 `9517`

#### Docker

```bash
docker run -d \
  --name file-download-proxy \
  -p 9517:9517 \
  --restart unless-stopped \
  shiodd/file-download-proxy:latest
```

启动后，访问 `http://localhost:9517` 出现 `success` 即为成功。

### Docker 构建

```bash
docker build -t myapp:latest .
```

