# Cloudflare Tunnel 代理服务

[![GitHub License](https://img.shields.io/badge/license-Apache%202.0-blue.svg
)](https://opensource.org/licenses/Apache-2.0)
[![Cloudflare兼容版本](https://img.shields.io/badge/cloudflared-v2025.2.0-green
)](https://github.com/cloudflare/cloudflared)

[English](README.md)

通过Cloudflare Argo Tunnel实现两个私网的互联互通，支持TCP/UDP协议转发, 支持TUN设备。

## 🛠️ 安装步骤

### 1. 获取程序

```bash
git clone https://github.com/fmnx/cftun.git
cd cftun
go build
```

# Tunnel 服务配置说明

本文档介绍了如何使用 JSON 配置文件来部署 Tunnel 隧道服务。
配置文件分为两大部分：服务端配置和客户端配置，用户可以根据自己的需求进行调整。

---

## 配置文件结构

配置文件为 JSON 格式，主要包含以下两个部分：

- **server**：服务端相关配置
- **client**：客户端相关配置

### 1. 服务端配置 (`server`)

- **token**  
  用于服务端认证的令牌，控制台创建隧道后生成的token。  
  若无cloudflare帐号，可填入`quick`， 将会通过try.cloudflare.com申请一个临时域名。  
  临时域名服务端运行期间长期有效，当服务端关闭超过10分钟后将会失效，再次启动时域名将会发生改变。  
  注意：临时域名需要配合客户端的`global-url`使用，通过在每个隧道配置中设置`remote`指定转发地址。

- **edge-ips** (可选)  
  指定服务端优选IP列表，下列为支持范围，端口为`7844`。
  ```yaml
  198.41.192.0/20
  2606:4700:a0::/48
  2606:4700:a1::/48
  2606:4700:a8::/48
  2606:4700:a9::/48
  ```

- **ha-conn** (可选)  
  高可用 QUIC 连接数，根据网络环境进行适当配置。

- **bind-address** (可选)  
  指定服务端出口网卡的 IP 地址。如无特殊需求建议留空

- **warp** (可选)  
  服务端出口添加warp双栈支持，基于wireguard。

    - **auto** (可选)  
      是否自动申请warp.默认值为`false` [true|false]

    - **port** (可选)  
      wireguard 本地监听端口。

    - **endpoint** (可选)  
      wireguard 终端。当`auto`为`false`时，此项必填。

    - **ipv4** (可选)  
      wireguard ipv4地址。当`auto`为`false`时，此项必填。

    - **ipv6** (可选)  
      wireguard ipv6地址。

    - **reserved** (可选)  
      设置warp的wireguard保留字段。

    - **private-key** (可选)  
      wireguard 私钥。当`auto`为`false`时，此项必填。

    - **public-key** (可选)  
      wireguard 公钥。当`auto`为`false`时，此项必填。

    - **proxy4** (可选)  
      出口是否使用warp代理ipv4流量. [true|false]

    - **proxy6** (可选)  
      出口是否使用warp代理ipv6流量. [true|false]

### 2. 客户端配置 (`client`)

- **cdn-ip** (可选)  
  优选 Cloudflare Anycast IP，如果不设置则解析url中的域名。

- **cdn-port** (可选)  
  CDN 的端口设置。ws标准端口为 `80`，wss标准端口为 `443`，默认为443端口。

- **scheme** (可选)  
  协议方案，支持 `ws` 或 `wss`，默认为wss，使用非标准端口时需根据实际情况设置。

- **global-url** (可选)  
  Tunnel控制台配置路径，如果存在 path，请一并填写。

- **tun** (可选)  
  Tun设备配置。

    - **enable** (可选)  
      是否启用tun设备，默认为否。[true|false]

    - **name** (可选)  
      tun设备名，默认为`cftun0`。

    - **ipv4** (可选)  
      自定义tun设备ipv4地址。

    - **ipv6** (可选)  
      自定义tun设备ipv6地址。

    - **mtu** (可选)  
      自定义tun设备MTU大小。

    - **interface** (可选)  
      tun设备指定出口网卡，默认为系统主网卡。

    - **log-level** (可选)  
      tun设备日志级别，[debug|info|warn|error|silent], 默认为`info`。

    - **routes** (可选)  
      tun设备路由匹配规则。

    - **ex-routes** (可选)  
      tun设备路由排除规则。

- **tunnels** (可选)  
  隧道配置列表，每个隧道包含以下配置：

    - **listen** (必选)  
      本地监听地址及端口, 建议使用127.0.0.1。

    - **remote** (可选)  
      转发到指定的目标地址(留空则使用控制台配置的目标地址)

    - **url** (可选)  
      优先使用该项配置(留空则使用`global-url`)。

    - **protocol** (可选)  
      指定隧道使用的协议，支持 `tcp` 或 `udp`，默认为`tcp`。

    - **timeout** (可选)  
      UDP 连接的超时时间（单位：秒），默认为 60 秒，如需调整可单独配置。

---

## 示例配置文件

### 以下是server示例配置文件：

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

### 以下是client使用tun模式配置文件：

```json
{
  "client": {
    "cdn-ip": "104.17.143.163",
    "cdn-port": 80,
    "scheme": "ws",
    "global-url": "argo.s01.dev",
    "tun": {
      "enable": true,
      "name": "tun1",
      "interface": "eth0",
      "log-level": "error",
      "routes": [
        "0.0.0.0/0",
        "::/1"
      ]
    }
  }
}
```

### 以下是client使用隧道模式配置文件：

```json
{
  "client": {
    "cdn-ip": "104.17.143.163",
    "cdn-port": 80,
    "scheme": "ws",
    "global-url": "argo.s01.dev",
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
