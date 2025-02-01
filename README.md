# Cloudflare Tunnel 代理服务

[![GitHub License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT)
![Cloudflare兼容版本](https://img.shields.io/badge/cloudflared-v2023.7.3-green)

通过Cloudflare Tunnel实现内网服务的安全暴露，支持TCP/UDP混合协议转发。基于[fmnx/cloudflared](https://github.com/fmnx/cloudflared)定制开发的客户端。

## 📦 前置要求

- 此处默认您已知悉如何在cloudflare web控制台配置tunnel
- 若要支持UDP，cloudflared续使用修改版：https://github.com/fmnx/cloudflared

## 🛠️ 安装步骤

### 1. 获取程序
```bash
git clone https://github.com/fmnx/cftun.git
cd cftun
go build
```

### 2. 配置文件
    - 配置文件中全局host搭配tunnel path使用，也可为tunnel独立设置host
```json5
{
  "cdn_ip": "104.20.20.20",           // 可选，手动指定的Cloudflare Anycast IP
  "host": "tunnel.s01.dev",           // 必填，tunnel默认域名
  "tunnels": [                        // 必填，配置隧道信息
    {                                 // 通过独立host定位
      "listen": "127.0.0.1:2222",     // 必填，本地监听地址
      "protocol": "tcp",              // 必填，支持tcp/udp
      "host": "ssh.s01.dev",          // 可选，URL路径标识
    },
    {                                 // 通过独立host+path定位
      "listen": "127.0.0.1:2223",     // 必填，本地监听地址
      "protocol": "tcp",              // 必填，支持tcp/udp
      "host": "s02.dev",              // 可选，URL路径标识
      "path": "ssh2"
    },
    {                                 // 通过全局host+path定位
      "listen": "127.0.0.1:5201",     // 必填，本地监听地址
      "protocol": "tcp",              // 必填，支持tcp/udp
      "path": "iperf3-tcp",           // 可选，URL路径标识
      "timeout": 30                   // 可选，UDP空闲超时(秒)
    },
    {                                 // 通过全局host+path定位
      "listen": "127.0.0.1:5201",     // 必填，本地监听地址
      "protocol": "udp",              // 必填，支持tcp/udp
      "path": "iperf3-udp",           // 可选，URL路径标识
      "timeout": 30                   // 可选，UDP空闲超时(秒)
    },
  ]                   
}
```

