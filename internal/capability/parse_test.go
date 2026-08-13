package capability

import "testing"

func TestParseOSReleaseHandlesQuotesEscapesAndMalformedLines(t *testing.T) {
	t.Parallel()

	input := []byte("# comment\nID=ubuntu\nPRETTY_NAME=\"Example \\\"Linux\\\"\"\nEMPTY=\nmalformed\n")

	got := ParseOSRelease(input)

	if got["ID"] != "ubuntu" {
		t.Fatalf("ID = %q, want ubuntu", got["ID"])
	}
	if got["PRETTY_NAME"] != `Example "Linux"` {
		t.Fatalf("PRETTY_NAME = %q", got["PRETTY_NAME"])
	}
	if got["EMPTY"] != "" {
		t.Fatalf("EMPTY = %q, want empty", got["EMPTY"])
	}
	if _, exists := got["malformed"]; exists {
		t.Fatal("malformed line unexpectedly parsed")
	}
}

func TestParseSelfStatusKeepsValidFieldsWhenOptionalFieldIsMalformed(t *testing.T) {
	t.Parallel()

	input := []byte("Name:\tasset-agent\nUid:\t0\t0\t0\t0\nGid:\t1000\t1000\t1000\t1000\nCapEff:\t0000000000003fff\nCapBnd:\tbroken\n")

	got := ParseSelfStatus(input)

	if got.UIDs != [4]uint32{0, 0, 0, 0} {
		t.Fatalf("UIDs = %v", got.UIDs)
	}
	if got.GIDs != [4]uint32{1000, 1000, 1000, 1000} {
		t.Fatalf("GIDs = %v", got.GIDs)
	}
	if got.CapEff != 0x3fff {
		t.Fatalf("CapEff = %#x, want 0x3fff", got.CapEff)
	}
	if got.CapBnd != 0 {
		t.Fatalf("CapBnd = %#x, want zero for malformed value", got.CapBnd)
	}
}
