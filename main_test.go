package main

import "testing"

func TestMain(t *testing.T) {
	if Greeting() != "hello world" {
		t.Fatalf("expected greeting to be hello world, it was %v", Greeting())
	}
}
