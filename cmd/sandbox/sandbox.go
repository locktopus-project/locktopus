package main

import (
	"fmt"
)

func main() {
	a := make(map[string]interface{})

	for i := 0; i <= 255; i++ {
		fmt.Println(i)
		for j := 0; j <= 255; j++ {
			for k := 0; k <= 255; k++ {
				for l := 0; l <= 255; l++ {
					key := string([]byte{byte(l), byte(k), byte(j), byte(i)})
					// key := string([]byte{byte(i), byte(j), byte(k), byte(l)})

					// fmt.Println(key)

					if v, ok := a[key]; ok {
						fmt.Printf("key %s already exists: %d, %d, %d, %d\n", key, i, j, k, l)
						fmt.Println(v)
						return
					}
					// time.Sleep(time.Second)
					a[key] = struct{}{}
				}
			}
		}
	}

}
