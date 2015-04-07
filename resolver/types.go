//go:generate go-extpoints . HostResolverConfig

package resolver

type HostResolverConfig interface {
	StoreAddress(address string) error
	Clean()
}
