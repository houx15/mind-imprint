package materialize

import "net"

// isBlockedIP reports whether ip must never be dialed (SSRF protection):
// loopback, private, link-local (incl. 169.254.169.254 cloud metadata),
// unspecified, multicast, or anything not global-unicast.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	return !ip.IsGlobalUnicast()
}
