package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/fmnx/cftun/log"
	"github.com/tidwall/gjson"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"
)

type Warp struct {
	Auto       bool   `yaml:"auto" json:"auto"`
	Endpoint   string `yaml:"endpoint" json:"endpoint"`
	IPv4       string `yaml:"ipv4" json:"ipv4"`
	IPv6       string `yaml:"ipv6" json:"ipv6"`
	PrivateKey string `yaml:"private-key" json:"private-key"`
	PublicKey  string `yaml:"public-key" json:"public-key"`
	Proxy4     bool   `yaml:"proxy4" json:"proxy4"`
	Proxy6     bool   `yaml:"proxy6" json:"proxy6"`
}

func (w *Warp) verify() bool {
	return w.Endpoint != "" && w.IPv4 != "" && w.PrivateKey != "" && w.PublicKey != ""
}

func (w *Warp) load() {
	buf, err := os.ReadFile(".warp.json")
	if err != nil {
		w.apply()
		w.save()
	} else {
		proxy4, proxy6 := w.Proxy4, w.Proxy6
		_ = json.Unmarshal(buf, w)
		w.Proxy4, w.Proxy6 = proxy4, proxy6
	}

}

func (w *Warp) save() {
	// 将内存中的数据静态化
	warpFile, _ := json.MarshalIndent(w, "", "  ")
	err := os.WriteFile(".warp.json", warpFile, 0644)
	if err != nil {
		log.Errorln("Error writing warp config file: %v", err)
		return
	}
}

func (w *Warp) apply() {

	log.Infoln("Automatically applying for Warp...")

	url := "https://api.cloudflareclient.com/v0a2223/reg"

	privateKey := NewPrivateKey()
	// 请求头
	headers := map[string]string{
		"CF-Client-Version": "a-6.11-2223",
		"Host":              "api.cloudflareclient.com",
		"Connection":        "Keep-Alive",
		"Accept-Encoding":   "gzip",
		"User-Agent":        "okhttp/3.12.1",
		"Content-Type":      "application/json",
	}

	jsonData, _ := json.Marshal(map[string]string{
		"key":    privateKey.Public().String(),
		"locale": "en-US",
		"tos":    time.Now().Format(time.RFC3339Nano),
	})

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln("A request error occurred while automatically applying for Warp.: %v", err)
	}
	defer resp.Body.Close()

	var reader io.ReadCloser
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, _ = gzip.NewReader(resp.Body)
		defer reader.Close()
	} else {
		reader = resp.Body
	}

	body, _ := io.ReadAll(reader)
	ipv6 := gjson.Get(string(body), "config.interface.addresses.v6").String()
	if ipv6 == "" {
		log.Fatalln("Failed to automatically apply for Warp.")
	}
	w.IPv4 = "172.16.0.2"
	w.IPv6 = ipv6
	w.PrivateKey = privateKey.String()
	w.PublicKey = "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo="
	w.Endpoint = "engage.cloudflareclient.com:2408"

	log.Infoln("Warp has been successfully applied.")
}

func (w *Warp) Run() {

	if w.Auto {
		w.load()
	} else if !w.verify() {
		log.Fatalln("The warp parameter is incorrect.")
	}

	if strings.Contains(w.IPv4, "/") {
		w.IPv4 = strings.Split(w.IPv4, "/")[0]
	}
	if strings.Contains(w.IPv6, "/") {
		w.IPv6 = strings.Split(w.IPv6, "/")[0]
	}

	localAddress := []netip.Addr{netip.MustParseAddr(w.IPv4)}
	if w.IPv6 != "" {
		localAddress = append(localAddress, netip.MustParseAddr(w.IPv6))
	}
	tunDev, tnet, err := netstack.CreateNetTUN(
		localAddress,
		[]netip.Addr{},
		1280,
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	bind := conn.NewStdNetBind()
	logger := device.NewLogger(1, "")
	dev := device.NewDevice(tunDev, bind, logger)
	dev.SetPrivateKey(w.PrivateKey)
	peer := dev.SetPublicKey(w.PublicKey)

	dev.SetEndpoint(peer, resolvEndpoint(w.Endpoint)).SetAllowedIP(peer)
	peer.HandlePostConfig()

	ingress.Warp.Set(tnet.DialContext, w.Proxy4, w.Proxy6)
}

func resolvEndpoint(endpoint string) string {
	host, port, _ := net.SplitHostPort(endpoint)
	c, err := net.DialTimeout("tcp6", "[2606:4700:4700::64]:443", 3*time.Second)
	defer c.Close()
	if err != nil { // ipv4-only
		addr, err := net.ResolveIPAddr("ip4", host)
		if err != nil {
			log.Fatalln("Failed to resolve endpoint for warp.")
		}
		return fmt.Sprintf("%s:%s", addr.String(), port)
	}
	addr, err := net.ResolveIPAddr("ip6", host)
	if err != nil {
		log.Fatalln("Failed to resolve endpoint for warp.")
	}
	return fmt.Sprintf("[%s]:%s", addr.String(), port)
}
