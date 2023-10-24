//go:build fips_enabled
// +build fips_enabled

// FIXME(rhmdnd): This was copied from openshift/boilerplate. We should
// consider migrating our `make` targets to using boilerplate, which include
// handy approaches and tools to enabling things consistently across
// operators.

package manager

import (
	_ "crypto/tls/fipsonly"
	"fmt"
)

func init() {
	fmt.Println("***** Starting with FIPS crypto enabled *****")
}
