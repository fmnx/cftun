# Cloudflare Tunnel 代理服务

[![GitHub License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT)
![Cloudflare兼容版本](https://img.shields.io/badge/cloudflared-v2023.7.3-green)

通过Cloudflare Tunnel实现内网服务的安全暴露，支持TCP/UDP混合协议转发。基于[修改版cloudflared](https://github.com/fmnx/cloudflared)定制开发的客户端。

## 📦 前置要求

- 此处默认您已知悉如何在cloudflare web控制台配置tunnel
- 由于控制台目前不支持设置UDP，控制台请选择RDP代替

## 🛠️ 安装步骤

### 1. 获取程序
```bash
git clone https://github.com/fmnx/cftun.git
cd cftun
go build
```

# Tunnel 服务配置说明

本文档介绍了如何使用 JSON 配置文件来部署 Tunnel 隧道服务。配置文件分为两大部分：服务端配置和客户端配置，用户可以根据自己的需求进行调整。

---

## 配置文件结构

配置文件为 JSON 格式，主要包含以下两个部分：

- **server**：服务端相关配置
- **client**：客户端相关配置

### 1. 服务端配置 (`server-config`)

- **token**  
  用于服务端认证的令牌，控制台创建隧道后生成的token。

- **edge-ips** (可选)  
  指定服务端优选 IP 列表，建议范围为 `198.41.192.0` 至 `198.41.207.255`，标准端口为 `7844`。
  此项支持使用支持UDP的优质线路中转至非标准端口。

- **ha-conn** (可选)  
  高可用 QUIC 连接数，根据网络环境进行适当配置。

- **bind-address** (可选)  
  指定服务端出口网卡的 IP 地址。如无特殊需求建议留空

### 2. 客户端配置 (`client-config`)

- **cdn-ip** (可选)  
  优选 Cloudflare Anycast IP，如果不设置则解析url中的域名。

- **cdn-port** (可选)  
  CDN 的端口设置。对于 WebSocket，ws标准端口为 `80`，wss标准端口为 `443`，支持使用优质线路中转至非标准端口。

- **scheme** (可选)  
  协议方案，支持 `ws` 或 `wss`，使用非标准端口时需根据实际情况设置。

- **tunnels** (可选)  
  隧道配置列表，每个隧道包含以下配置：

    - **listen** (必选)  
      本地监听地址及端口, 建议使用127.0.0.1。

    - **url** (必选)  
      控制台设置的 URL（如果存在 path，请一并填写）。

    - **protocol** (必选)  
      指定本地监听端口使用的协议，支持 `tcp` 或 `udp`。

    - **timeout** (可选)  
      UDP 连接的超时时间（单位：秒），默认为 60 秒，如需调整可单独配置。

---

## 示例配置文件

以下是一个示例配置文件，您可以直接复制并根据实际需求进行修改：

```json
{
  "server": {
    "token": "",
    "edge-ips": [
      "198.41.192.77:7844",
      "198.41.197.78:7844",
      "198.41.202.79:7844",
      "198.41.207.80:7844"
    ],
    "ha-conn": 4,
    "bind-address": ""
  },
  "client": {
    "cdn-ip": "104.17.143.163",
    "cdn-port": 80,
    "scheme": "ws",
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
