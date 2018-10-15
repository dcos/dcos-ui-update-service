package main

import "fmt"

func Greeting() string {
	return "hello world"
}

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	fmt.Println(Greeting())
}
