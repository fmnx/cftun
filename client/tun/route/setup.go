package route

func ConfigureTun(tunName, ipv4, ipv6 string, routes, exRoutes []string) {
	configureAddress(tunName, ipv4, ipv6)
	configureRoute(tunName, ipv4, ipv6, routes, exRoutes)
}

func configureAddress(tunName, ipv4, ipv6 string) {
	configureAddressImpl(tunName, ipv4, ipv6)
}

func configureRoute(tunName, ipv4, ipv6 string, routes, exRoutes []string) {
	configureRouteImpl(tunName, ipv4, ipv6, routes, exRoutes)
}
