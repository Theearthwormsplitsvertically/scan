package linux

import (
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
	"github.com/Theearthwormsplitsvertically/scan/internal/provider"
)

func TestNewRegistersLinuxProviders(t *testing.T) {
	t.Parallel()

	set, err := New(platform.NewRoot(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if set.Platform() != "linux" {
		t.Fatalf("platform = %q", set.Platform())
	}
	for _, capability := range []string{
		provider.CapabilitySystemProfile,
		provider.CapabilityHost,
		provider.CapabilityNetwork,
		provider.CapabilityProcess,
		provider.CapabilitySocket,
	} {
		if _, ok := set.Lookup(capability); !ok {
			t.Errorf("capability %q is not registered", capability)
		}
	}

	assertTypedProvider[provider.ProfileProvider](t, set, provider.CapabilitySystemProfile)
	assertTypedProvider[provider.HostProvider](t, set, provider.CapabilityHost)
	assertTypedProvider[provider.NetworkProvider](t, set, provider.CapabilityNetwork)
	assertTypedProvider[provider.ProcessProvider](t, set, provider.CapabilityProcess)
	assertTypedProvider[provider.SocketProvider](t, set, provider.CapabilitySocket)
}

func assertTypedProvider[T provider.Provider](t *testing.T, set provider.Lookup, capability string) {
	t.Helper()
	if _, ok := provider.As[T](set, capability); !ok {
		t.Errorf("capability %q does not implement its typed contract", capability)
	}
}
