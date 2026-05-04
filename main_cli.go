//go:build cli

package main

import (
	"flag"

	"zun-talk/config"
)

func main() {
	charKey := flag.String("char", config.DefaultCharKey, "キャラクター選択: zundamon / tsumugi / metan")
	flag.Parse()
	runCLI(*charKey)
}
