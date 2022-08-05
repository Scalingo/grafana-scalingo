package main

import (
	"fmt"
	"os"
	"strconv"

	apikeygenprefix "github.com/grafana/grafana/pkg/components/apikeygenprefixed"
)

// placeholder key generator
func main() {
	// get number of keys to generate from args
	numKeys := 1
	if len(os.Args) > 1 {
		var err error
		numKeys, err = strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Println("ERROR: invalid number of keys to generate:", err)
			return
		}
	}

	for i := 0; i < numKeys; i++ {
		key, err := apikeygenprefix.New("pl")
		if err != nil {
			fmt.Println("ERROR: generating key failed:", err)
			return
		}

		fmt.Printf("\nGenerated key: %d:\n", i+1)
		fmt.Println(key.ClientSecret)
		fmt.Printf("\nGenerated key hash: %d \n", i+1)
		fmt.Println(key.HashedKey)
	}
}
