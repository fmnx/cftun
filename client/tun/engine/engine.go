package engine

import (
	"github.com/fmnx/cftun/client/tun/buffer"
	"github.com/fmnx/cftun/client/tun/dialer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"net"
	"net/netip"
	"sync"

	"github.com/fmnx/cftun/client/tun/core"
	"github.com/fmnx/cftun/client/tun/core/device"
	"github.com/fmnx/cftun/client/tun/core/option"
	"github.com/fmnx/cftun/client/tun/log"
	"github.com/fmnx/cftun/client/tun/proxy"
	"github.com/fmnx/cftun/client/tun/tunnel"
)

var (
	Mu sync.Mutex

	// Device holds the default device for the engine.
	Device device.Device

	// Stack holds the default stack for the engine.
	Stack *stack.Stack
)

// Stop shuts the default engine down.
func Stop() {
	if err := stop(); err != nil {
		log.Fatalf("[ENGINE] failed to stop: %v", err)
	}
}

func stop() (err error) {
	Mu.Lock()
	if Device != nil {
		Device.Close()
	}
	if Stack != nil {
		Stack.Close()
		Stack.Wait()
	}
	Mu.Unlock()
	return nil
}

func HandleNetStack(argoProxy *proxy.Argo, device, interfaceName, logLevel string, mtu int) (err error) {
	buffer.RelayBufferSize = mtu
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	log.SetLogger(log.Must(log.NewLeveled(level)))

	if interfaceName != "" {
		iface, err := net.InterfaceByName(interfaceName)
		if err != nil {
			return err
		}
		dialer.DefaultInterfaceName.Store(iface.Name)
		dialer.DefaultInterfaceIndex.Store(int32(iface.Index))
		log.Infof("[DIALER] bind to interface: %s", interfaceName)
	}

	transport := tunnel.New(argoProxy)
	transport.ProcessAsync()

	if Device, err = parseDevice(device, uint32(mtu)); err != nil {
		log.Fatalf(err.Error(), "\n")
		return
	}

	var multicastGroups []netip.Addr
	//if multicastGroups, err = parseMulticastGroups(MulticastGroups); err != nil {
	//	return err
	//}

	var opts []option.Option
	//if TCPModerateReceiveBuffer {
	//	opts = append(opts, option.WithTCPModerateReceiveBuffer(true))
	//}

	//if TCPSendBufferSize != "" {
	//	size, err := units.RAMInBytes(TCPSendBufferSize)
	//	if err != nil {
	//		return err
	//	}
	//	opts = append(opts, option.WithTCPSendBufferSize(int(size)))
	//}

	//if TCPReceiveBufferSize != "" {
	//	size, err := units.RAMInBytes(TCPReceiveBufferSize)
	//	if err != nil {
	//		return err
	//	}
	//	opts = append(opts, option.WithTCPReceiveBufferSize(int(size)))
	//}

	if Stack, err = core.CreateStack(&core.Config{
		LinkEndpoint:     Device,
		TransportHandler: transport,
		MulticastGroups:  multicastGroups,
		Options:          opts,
	}); err != nil {
		return
	}

	log.Infof(
		"[STACK] %s://%s <-> %s -> %s",
		Device.Type(), Device.Name(),
		argoProxy.Host(), argoProxy.Addr(),
	)
	return nil
}
