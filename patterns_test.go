package umbra_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/feds01/umbra"
)

const hidden = "<null>"

// capture is what a view shows of input: the pattern's group in lower case, or NULL.
func capture(pattern, input string) string {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if matches == nil {
		return hidden
	}

	return strings.ToLower(matches[1])
}

func TestEmailPatternKeepsTheDomainOfOneWellFormedAddress(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"one address", "Alice@Example.COM", "example.com"},
		{"a subdomain", "a.b@mail.eu.example.org", "mail.eu.example.org"},
		{"a hyphenated domain", "a@my-company.example.co.uk", "my-company.example.co.uk"},
		{"a list of addresses", "alice@a.example,bob.LISTSECRET@b.example", hidden},
		{"two addresses with a space", "alice@a.example bob.LISTSECRET@b.example", hidden},
		{"a second at sign", "alice@bob.LISTSECRET@b.example", hidden},
		{"a display name", "Alice <alice@a.example>", hidden},
		{"no domain", "alice@", hidden},
		{"no address", "not an address", hidden},
		{"a name with no dot", "admin@HUNTER2SECRET", hidden},
		{"a token with no dot", "token@sk-LIVE-abc123SECRET", hidden},
		{"a url with an address in its query", "https://x.example/?next=a@LISTSECRET.example", hidden},
		{"a connection string", "postgres://admin:hunter2@DBHOSTSECRET.internal", hidden},
		{"a pipe in the local part", "alice|bob@b.example", hidden},
		{"an empty label", "alice@a..LISTSECRET", hidden},
		{"a trailing dot", "alice@a.example.", hidden},
		{"a newline", "alice@a.example\nbob@LISTSECRET.example", hidden},
		{"a tab", "alice@a.example\tbob@LISTSECRET.example", hidden},
		{"a no-break space", "alice@a.example\u00a0bob@LISTSECRET.example", hidden},
		{"a full-width comma", "alice@a.example\uff0cbob@LISTSECRET.example", hidden},
		{"a zero-width space", "alice@a.example\u200bbob@LISTSECRET.example", hidden},
		{"a percent-encoded comma", "alice@a.example%2Cbob@LISTSECRET.example", hidden},
	}

	for _, test := range tests {
		if got := capture(umbra.EmailPattern, test.input); got != test.want {
			t.Errorf("%s: got %s, want %s", test.name, got, test.want)
		}
	}
}

func TestURLPatternKeepsTheHostOfOneWellFormedURL(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"path and query", "https://Canary.Example/PATHSECRET?q=QUERYSECRET#f", "canary.example"},
		{"userinfo and port", "https://user:pw@Host.Example:8443/p?q=1", "host.example"},
		{"no path", "http://host.example", "host.example"},
		{"an address in the query", "https://x.example/?next=a@LISTSECRET.example", "x.example"},
		{"an ipv6 literal", "http://[::1]:80/x", "[::1]"},
		{"a compressed ipv6 address", "https://[2001:db8::1]/x", "[2001:db8::1]"},
		{"a full ipv6 address", "https://[2001:db8:0:0:0:0:0:1]:8443/x", "[2001:db8:0:0:0:0:0:1]"},
		{"a hex token behind a colon", "https://[:deadbeefcafe]/x", hidden},
		{"a hex token before a colon", "https://[deadbeef:]/x", hidden},
		{"a dot inside the brackets", "https://[a.b:c]/", hidden},
		{"too few groups", "https://[1:2:3]/", hidden},
		{"a group of five digits", "https://[12345::1]/", hidden},
		{"a second at sign in the userinfo", "https://admin:hunter2@PASSSECRET@intranet.example/x", hidden},
		{"a list of hosts", "https://a.example,PASSSECRET.example/x", hidden},
		{"two ports", "https://a.example:80:90/x", hidden},
		{"no scheme", "a.example/PATHSECRET", hidden},
		{"no host", "https:///PATHSECRET", hidden},
		{"a connection string", "postgres://admin:hunter2@DBHOSTSECRET.internal", hidden},
		{"a path that looks like userinfo", "https://alice:123/PATHSECRET@host.example/", hidden},
		{"a username and port", "https://ALICE:8443", hidden},
		{"a host with no dot", "https://SECRETTOKEN/x", hidden},
		{"localhost", "http://localhost:3000", hidden},
		{"a bucket", "s3://BUCKETSECRET/key", hidden},
		{"a file", "file://SECRET/etc/passwd", hidden},
		{"brackets that are not ipv6", "https://[deadbeefcafe]/x", hidden},
		{"a backslash in the userinfo", "https://host.example\\@PASSSECRET.example/", hidden},
		{"an empty label", "https://a..LISTSECRET/x", hidden},
		{"a newline in the path", "https://a.example/x\nhttps://LISTSECRET.example", hidden},
		{"a full-width slash", "https://a.example\uff0fLISTSECRET.example", hidden},
		{"a zero-width space", "https://a.example\u200bLISTSECRET.example/", hidden},
		{"a percent-encoded slash", "https://a.example%2FLISTSECRET.example/", hidden},
	}

	for _, test := range tests {
		if got := capture(umbra.URLPattern, test.input); got != test.want {
			t.Errorf("%s: got %s, want %s", test.name, got, test.want)
		}
	}
}
