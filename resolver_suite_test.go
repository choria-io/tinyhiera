// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package tinyhiera

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTinyhiera(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TinyHiera Resolver Suite")
}
