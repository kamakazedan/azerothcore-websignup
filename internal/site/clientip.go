package site

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Proxies decides which X-Forwarded-For entries to believe.
//
// Behind Nginx Proxy Manager every request comes from the proxy's address;
// the player's own is the last entry in X-Forwarded-For that was not added
// by a proxy we trust. A request straight from the internet gets its
// RemoteAddr, whatever headers it brings.
type Proxies []netip.Prefix

func ParseProxies(list string) (Proxies, error) {
	var out Proxies
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			if a, err := netip.ParseAddr(s); err == nil {
				s = netip.PrefixFrom(a, a.BitLen()).String()
			}
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func (p Proxies) trusted(a netip.Addr) bool {
	a = a.Unmap()
	for _, pre := range p {
		if pre.Contains(a) {
			return true
		}
	}
	return false
}

// limitKey is what the per-address limits count by: an IPv4 address, or
// an IPv6 address's /64, since one home or server gets a whole /64 and
// could otherwise use a fresh address for every request.
func limitKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	a = a.Unmap()
	if a.Is4() {
		return a.String()
	}
	return netip.PrefixFrom(a, 64).Masked().String()
}

// ClientIP is the address rate limits and logs use.
func (p Proxies) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	remote = remote.Unmap()
	if !p.trusted(remote) {
		return remote.String()
	}

	// Walk X-Forwarded-For from the right: the first address we do not
	// trust is the client.
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		a = a.Unmap()
		if !p.trusted(a) {
			return a.String()
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		if a, err := netip.ParseAddr(real); err == nil {
			return a.Unmap().String()
		}
	}
	return remote.String()
}
