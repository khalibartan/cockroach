// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/cockroachdb/cockroach/pkg/cmd/urlcheck/lib/urlcheck"
)

func main() {
	cmd := exec.Command("git", "grep", "-nE", urlcheck.URLRE)
	if err := urlcheck.CheckURLsFromGrepOutput(cmd); err != nil {
		log.Fatalf("%+v\nFAIL", err)
	}
	fmt.Println("PASS")
}
