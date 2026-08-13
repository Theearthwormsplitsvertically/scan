package socket

import (
	"strings"
	"testing"
)

func TestParseProcNetNormalizesIPv4TCP(t *testing.T) {
	t.Parallel()

	fixture := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt uid timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 12345\n" +
		"   1: 0100007F:C350 08080808:01BB 01 00000000:00000000 00:00000000 00000000 0 0 12346\n"

	sockets, errs := ParseProcNet(strings.NewReader(fixture), "tcp", 4, "net:[1]")

	if len(errs) != 0 || len(sockets) != 2 {
		t.Fatalf("sockets/errors = %d/%v", len(sockets), errs)
	}
	if got := sockets[0]; got.LocalAddress != "127.0.0.1" || got.LocalPort != 8080 || got.State != "LISTEN" || got.Inode != 12345 {
		t.Fatalf("listen socket = %+v", got)
	}
	if got := sockets[1]; got.RemoteAddress != "8.8.8.8" || got.RemotePort != 443 || got.State != "ESTABLISHED" {
		t.Fatalf("connected socket = %+v", got)
	}
}

func TestParseProcNetNormalizesIPv6LoopbackAndSkipsMalformedRows(t *testing.T) {
	t.Parallel()

	fixture := "sl local_address rem_address st queues timer retransmit uid timeout inode\n" +
		"0: 00000000000000000000000001000000:01BB 00000000000000000000000000000000:0000 0A 0:0 0:0 0 0 0 456\n" +
		"broken row\n"

	sockets, errs := ParseProcNet(strings.NewReader(fixture), "tcp", 6, "net:[2]")

	if len(sockets) != 1 || sockets[0].LocalAddress != "::1" || sockets[0].LocalPort != 443 {
		t.Fatalf("sockets = %+v", sockets)
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %d, want 1", len(errs))
	}
}
