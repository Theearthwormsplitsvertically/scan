package network

import (
	"strings"
	"testing"
)

func TestParseIPv4RoutesConvertsLinuxLittleEndianAddresses(t *testing.T) {
	t.Parallel()

	fixture := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0001A8C0\t00000000\t0001\t0\t0\t50\t00FFFFFF\t0\t0\t0\n" +
		"broken\trow\n"

	routes, errs := ParseIPv4Routes(strings.NewReader(fixture))

	if len(routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(routes))
	}
	if got := routes[0]; got.Interface != "eth0" || got.Destination != "0.0.0.0/0" || got.Gateway != "192.168.1.1" || got.Metric != 100 {
		t.Fatalf("default route = %+v", got)
	}
	if got := routes[1]; got.Destination != "192.168.1.0/24" || got.Gateway != "0.0.0.0" || got.Metric != 50 {
		t.Fatalf("subnet route = %+v", got)
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %d, want 1", len(errs))
	}
}
