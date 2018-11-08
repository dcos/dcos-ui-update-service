package main

import (
	"strings"
	"testing"
)

func TestDCOS(t *testing.T) {
	t.Run("Multi-Master", func(t *testing.T) {
		t.Parallel()

		// TODO: move to table driven tests
		t.Run("throws if the file is not found", func(t *testing.T) {
			system := DCOS{
				MasterCountLocation: "fixtures/non-existant",
			}

			result, err := system.IsMultiMaster()
			if err == nil {
				t.Fatalf("no error was thrown, although expected. Instead got result %t", result)
			}

			if !strings.Contains(err.Error(), "Could not find") {
				t.Fatalf("Error message should hint that it was not found. Instead got error message %q", err.Error())
			}
		})

		t.Run("throws if the file is empty", func(t *testing.T) {
			system := DCOS{
				MasterCountLocation: "fixtures/empty",
			}

			result, err := system.IsMultiMaster()
			if err == nil {
				t.Fatalf("no error was thrown, although expected. Instead got result %t", result)
			}

			if !strings.Contains(err.Error(), "could not be parsed") {
				t.Fatalf("Error message should show that it can not parse the file. Instead got error message %q", err.Error())
			}
		})

		t.Run("returns true if there is a number bigger than 1", func(t *testing.T) {
			system := DCOS{
				MasterCountLocation: "fixtures/multi-master",
			}

			result, err := system.IsMultiMaster()
			if err != nil {
				t.Fatalf("error was thrown, although none expected: %v", err)
			}

			if !result {
				t.Fatalf("got %t should have been %t", false, true)
			}
		})

		t.Run("returns false if there is only one master", func(t *testing.T) {
			system := DCOS{
				MasterCountLocation: "fixtures/single-master",
			}

			result, err := system.IsMultiMaster()
			if err != nil {
				t.Fatalf("error was thrown, although none expected: %v", err)
			}

			if result {
				t.Fatalf("got %t should have been %t", true, false)
			}
		})
	})
}
