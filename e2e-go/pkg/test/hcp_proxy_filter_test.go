package hypershift_test

import "testing"

func TestHCPProxyOnlyFilter(t *testing.T) {
	testCases := []struct {
		name   string
		filter string
		want   bool
	}{
		{name: "empty filter uses full bootstrap", filter: "", want: false},
		{name: "proxy label uses minimal bootstrap", filter: "hcp-proxy", want: true},
		{name: "proxy story label uses minimal bootstrap", filter: "ACM-37268", want: true},
		{name: "proxy labels combined with and use minimal bootstrap", filter: "hcp-proxy && ACM-39216", want: true},
		{name: "create label uses full bootstrap", filter: "create", want: false},
		{name: "destroy label uses full bootstrap", filter: "destroy", want: false},
		{name: "metrics label uses full bootstrap", filter: "metrics", want: false},
		{name: "mixed proxy and create uses full bootstrap", filter: "hcp-proxy || create", want: false},
		{name: "proxy with negated create uses minimal bootstrap", filter: "hcp-proxy && !create", want: true},
		{name: "proxy or negated create uses full bootstrap", filter: "hcp-proxy || !create", want: false},
		{name: "negated non-proxy label uses full bootstrap", filter: "!create", want: false},
		{name: "negated proxy label uses full bootstrap", filter: "!hcp-proxy", want: false},
		{name: "proxy with negated metrics uses minimal bootstrap", filter: "hcp-proxy && !metrics", want: true},
		{name: "regex filter uses full bootstrap", filter: "/create/", want: false},
		{name: "malformed filter uses full bootstrap", filter: "hcp-proxy &&", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hcpProxyOnlyFilter(tc.filter); got != tc.want {
				t.Fatalf("hcpProxyOnlyFilter(%q) = %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}
