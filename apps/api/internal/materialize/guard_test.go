package materialize

import (
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "::1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "169.254.169.254", "0.0.0.0", "fe80::1", "fc00::1"}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("expected %s to be allowed", s)
		}
	}
	if !isBlockedIP(nil) {
		t.Errorf("nil IP must be blocked")
	}
}
