package umbra_test

import (
	"fmt"

	"github.com/feds01/umbra"
)

// policy is a policy for a SaaS app that stores accounts, contacts and users. It has five
// reasons to deny a column, a basis that allows reviewed JSON, and three maskers. A
// pseudonym is keyed by organisation, so a table must allow org_id to pseudonymise anything.
func policy() umbra.Policy {
	return umbra.Policy{
		Reasons: []umbra.Reason{"credential", "person", "free_text", "json", "internal"},
		Bases:   []umbra.Basis{"business_data"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
			{Term: "generalise=host", Suffix: "host", Kinds: umbra.Text},
			{
				Term:   "pseudonymise",
				Suffix: "pseudonym",
				Kinds:  umbra.Text | umbra.UUID | umbra.Inet,
				Reason: "person",
				Reads:  []string{"org_id"},
			},
		},
		Rules: []umbra.Rule{businessDataOnlyOnAccounts},
	}
}

// businessDataOnlyOnAccounts keeps allow=business_data to the accounts table, which holds
// companies, not people.
func businessDataOnlyOnAccounts(dataset string, table umbra.Table) error {
	for _, column := range table.Columns {
		allow, isAllow := column.Class.(umbra.Allow)
		if isAllow && allow.Basis == "business_data" && table.Name != "accounts" {
			return fmt.Errorf("%s: allow=business_data is only for accounts", column.Name)
		}
	}

	return nil
}

func ExamplePolicy() {
	tags := []string{
		"allow",
		"allow=business_data",
		"deny=person",
		"generalise=domain,pseudonymise",
		"deny",
		"generalise=domian",
	}

	for _, tag := range tags {
		class, err := umbra.ParseTag(tag, policy())
		if err != nil {
			fmt.Println(err)

			continue
		}

		fmt.Printf("%-12T %s\n", class, class)
	}

	// Output:
	// umbra.Allow  allow
	// umbra.Allow  allow=business_data
	// umbra.Deny   deny=person
	// umbra.Mask   generalise=domain,pseudonymise
	// deny needs a reason: credential, person, free_text, json or internal
	// unknown term "generalise=domian"; use allow, deny=<reason>, generalise=domain, generalise=host or pseudonymise
}
