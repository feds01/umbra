package umbra

// Regular expressions for a masker that keeps part of a value. Each captures the part it
// keeps in its one group, and matches only a whole, well-formed value, so a list of
// addresses or a string that only looks like a URL yields nothing. They are POSIX
// enough for Postgres, and RE2 for BigQuery and Go.
const (
	// EmailPattern matches one address whose domain is a dotted name, and captures the domain.
	EmailPattern = `^[^@[:space:],;<>()/:?=|\\]+@(` + dnsName + `)$`

	// URLPattern matches one http or https URL, and captures its host: a dotted name or an
	// IPv6 literal.
	URLPattern = `^https?://(?:[^/?#@[:space:]\\]*@)?(` + dnsName + `|` + ipv6Literal + `)` +
		`(?::[0-9]+)?(?:[/?#].*)?$`
)

// dnsName is two or more dot-separated labels of letters, digits and inner hyphens.
const dnsName = `[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+`

// hexGroup is one group of an IPv6 address.
const hexGroup = `[0-9a-fA-F]{1,4}`

// ipv6Literal matches a bracketed address of eight groups, or fewer around one "::".
const ipv6Literal = `[[](?:` + hexGroup + `(?::` + hexGroup + `){7}` +
	`|(?:` + hexGroup + `(?::` + hexGroup + `){0,6})?::(?:` + hexGroup + `(?::` + hexGroup + `){0,6})?)[]]`
