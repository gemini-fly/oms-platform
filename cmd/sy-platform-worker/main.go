package main

import (
	"flag"
	"fmt"
)

func main() {
	queue := flag.String("queue", "default", "worker queue name")
	flag.Parse()
	fmt.Printf("sy-platform worker ready queue=%s\n", *queue)
}
