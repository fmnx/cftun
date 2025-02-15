# Cloudflare Tunnel Proxy Service

[![GitHub License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Cloudflare Compatible Version](https://img.shields.io/badge/cloudflared-v2025.2.0-green)](https://github.com/cloudflare/cloudflared)

[中文文档](README_ZH.md)

Securely expose internal network services through Cloudflare Tunnel with support for TCP/UDP hybrid protocol forwarding. Custom client based on [modified cloudflared](https://github.com/fmnx/cloudflared).

## 🛠️ Installation Steps

### 1. Get the Program
```bash
git clone https://github.com/fmnx/cftun.git
cd cftun
go build
```

# Tunnel Service Configuration

This document describes how to deploy the Tunnel service using a JSON configuration file. 
The configuration file is divided into two main sections: server and client. Users can adjust these according to their requirements.

---

## Configuration File Structure

The JSON configuration file contains two main sections:

- **server**：Server-related configurations
- **client**：Client-related configurations

### 1. Server Configuration (`server`)

- **token**  
  Authentication token for the server. Use the token generated after creating a tunnel in the Cloudflare dashboard.
  If you don't have a Cloudflare account, use `quick` to request a temporary domain via try.cloudflare.com.
  The temporary domain remains valid while the server is running. If the server stays offline for over 10 minutes, the domain will expire and change upon restart.
  Note: Temporary domains require using the client's `global-url` with `remote` specified in each tunnel configuration.

- **edge-ips** (optional)  
  Preferred IP list for the server. The following ranges are supported, with port `7844`.
  ```yaml
  198.41.192.0/20
  2606:4700:a0::/48
  2606:4700:a1::/48
  2606:4700:a8::/48
  2606:4700:a9::/48
  ```

- **ha-conn** (optional)  
  Number of high-availability QUIC connections. Adjust according to network environment.

- **bind-address** (optional)  
  Specify the server's egress network interface IP. Leave empty if not required.

### 2. Client Configuration (client)

- **cdn-ip** (optional)  
  Preferred Cloudflare Anycast IP. If empty, resolves the domain in the URL.

- **cdn-port** (optional)  
  CDN port settings. For WebSocket: standard ws port `80`, wss port `443`.

- **scheme** (optional)  
  Protocol scheme: `ws` or `wss`. Required when using non-standard ports.

- **global-url** (optional)  
  Tunnel dashboard configuration path. Include full path if applicable.

- **tunnels** (optional)  
  List of tunnel configurations:

    - **listen** (required)  
      Local listening address and port (recommend 127.0.0.1).

    - **remote** (optional)  
      Forward to specified target address (empty uses dashboard configuration).

    - **url** (optional)  
      Priority configuration (uses global-url if empty).

    - **protocol** (required)  
      Local protocol: tcp or udp.

    - **timeout** (optional)  
      UDP connection timeout in seconds (default: 60).

---

## Example Configurations

### Server configuration example:
```json
{
  "server": {
    "token": "quick",
    "edge-ips": [
      "198.41.192.77:7844",
      "198.41.197.78:7844",
      "198.41.202.79:7844",
      "198.41.207.80:7844"
    ],
    "ha-conn": 4,
    "bind-address": ""
  }
}
```

### Client configuration using global-url:
```json
{
  "client": {
    "cdn-ip": "104.17.143.163",
    "cdn-port": 80,
    "scheme": "ws",
    "global-url": "tunnel.qzzz.io",
    "tunnels": [
      {
        "listen": "127.0.0.1:2408",
        "remote": "162.159.192.1:2408",
        "protocol": "udp",
        "timeout": 30
      },
      {
        "listen": "127.0.0.1:2222",
        "remote": "127.0.0.1:22",
        "protocol": "tcp"
      },
      {
        "listen": "127.0.0.1:5201",
        "remote": "127.0.0.1:5201",
        "protocol": "udp",
        "timeout": 30
      },
      {
        "listen": "127.0.0.1:5201",
        "remote": "127.0.0.1:5201",
        "protocol": "tcp"
      }
    ]
  }
}
```

### Client configuration using individual url:
```json
{
  "client": {
    "cdn-ip": "104.17.143.163",
    "cdn-port": 80,
    "scheme": "ws",
    "global-url": "",
    "tunnels": [
      {
        "listen": "127.0.0.1:2408",
        "url": "warp.qzzz.io",
        "protocol": "udp",
        "timeout": 30
      },
      {
        "listen": "127.0.0.1:2222",
        "url": "ssh.qzzz.io",
        "protocol": "tcp"
      },
      {
        "listen": "127.0.0.1:5201",
        "url": "iperf3.qzzz.io/udp",
        "protocol": "udp",
        "timeout": 30
      },
      {
        "listen": "127.0.0.1:5201",
        "url": "iperf3.qzzz.io/tcp",
        "protocol": "tcp"
      }
    ]
  }
}
```