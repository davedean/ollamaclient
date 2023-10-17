package main

import (
	"fmt"
	"strings"

	"github.com/xyproto/ollamaclient"
)

func main() {
	oc := ollamaclient.New()

	oc.Verbose = true

	prompt := "Write a haiku about the color of cows."
	output, err := oc.GetOutput(prompt)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("\n%s\n", strings.TrimSpace(output))
}
