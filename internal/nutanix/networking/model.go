package networking

// Subnet is the reviewed getSubnetById projection used by Terraform state mapping.
type Subnet struct {
	ExtID                  *string     `json:"extId"`
	Name                   *string     `json:"name"`
	Description            *string     `json:"description"`
	SubnetType             *string     `json:"subnetType"`
	NetworkID              *int64      `json:"networkId"`
	IPConfig               *[]IPConfig `json:"ipConfig"`
	ClusterReference       *string     `json:"clusterReference"`
	VirtualSwitchReference *string     `json:"virtualSwitchReference"`
	VPCReference           *string     `json:"vpcReference"`
	IsNATEnabled           *bool       `json:"isNatEnabled"`
	IsExternal             *bool       `json:"isExternal"`
	BridgeName             *string     `json:"bridgeName"`
	IsAdvancedNetworking   *bool       `json:"isAdvancedNetworking"`
}

// IPConfig is the dual-stack IP configuration projection.
type IPConfig struct {
	IPv4 *IPv4Config `json:"ipv4"`
	IPv6 *IPv6Config `json:"ipv6"`
}

// IPv4Config is the reviewed IPv4 configuration projection.
type IPv4Config struct {
	IPSubnet          *IPv4Subnet  `json:"ipSubnet"`
	DefaultGatewayIP  *IPv4Address `json:"defaultGatewayIp"`
	DHCPServerAddress *IPv4Address `json:"dhcpServerAddress"`
	PoolList          *[]IPv4Pool  `json:"poolList"`
}

// IPv6Config is the reviewed IPv6 configuration projection.
type IPv6Config struct {
	IPSubnet          *IPv6Subnet  `json:"ipSubnet"`
	DefaultGatewayIP  *IPv6Address `json:"defaultGatewayIp"`
	DHCPServerAddress *IPv6Address `json:"dhcpServerAddress"`
	PoolList          *[]IPv6Pool  `json:"poolList"`
}

// IPv4Subnet is the reviewed IPv4 network projection.
type IPv4Subnet struct {
	IP           *IPv4Address `json:"ip"`
	PrefixLength *int64       `json:"prefixLength"`
}

// IPv6Subnet is the reviewed IPv6 network projection.
type IPv6Subnet struct {
	IP           *IPv6Address `json:"ip"`
	PrefixLength *int64       `json:"prefixLength"`
}

// IPv4Address is the reviewed IPv4 address projection.
type IPv4Address struct {
	Value        *string `json:"value"`
	PrefixLength *int64  `json:"prefixLength"`
}

// IPv6Address is the reviewed IPv6 address projection.
type IPv6Address struct {
	Value        *string `json:"value"`
	PrefixLength *int64  `json:"prefixLength"`
}

// IPv4Pool is the reviewed IPv4 pool projection.
type IPv4Pool struct {
	StartIP *IPv4Address `json:"startIp"`
	EndIP   *IPv4Address `json:"endIp"`
}

// IPv6Pool is the reviewed IPv6 pool projection.
type IPv6Pool struct {
	StartIP *IPv6Address `json:"startIp"`
	EndIP   *IPv6Address `json:"endIp"`
}
