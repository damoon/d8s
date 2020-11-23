package main

func main() {
	var data = [1968 * 1024 * 1024]byte{}
	for i := 0; i < len(data); i++ {
		data[i] = 1
	}
}
