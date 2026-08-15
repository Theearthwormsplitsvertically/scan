package service

import "testing"

func TestParseUnitExtractsStaticFacts(t *testing.T) {
	t.Parallel()

	data := []byte("[Unit]\nDescription=Web Server\nAfter=network.target\n\n[Service]\nExecStart=/usr/sbin/nginx\nUser=www-data\nGroup=www-data\n\n[Install]\nWantedBy=multi-user.target\n")
	unit, err := ParseUnit(data, "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if unit.UnitName != "nginx" || unit.Description != "Web Server" || unit.ExecStart != "/usr/sbin/nginx" ||
		unit.User != "www-data" || unit.Group != "www-data" || unit.WantedBy != "multi-user.target" {
		t.Fatalf("unit = %+v", unit)
	}
}

func TestParseUnitSkipsCommentsAndKeepsFirstExecStart(t *testing.T) {
	t.Parallel()

	data := []byte("# leading comment\n[Service]\nExecStart=/bin/true\nExecStart=/bin/ignored\nExecStop=/bin/false\n")
	unit, err := ParseUnit(data, "test")
	if err != nil {
		t.Fatal(err)
	}
	if unit.ExecStart != "/bin/true" {
		t.Fatalf("exec start = %q", unit.ExecStart)
	}
	if unit.Description != "" || unit.User != "" || unit.WantedBy != "" {
		t.Fatalf("unexpected fields = %+v", unit)
	}
}
