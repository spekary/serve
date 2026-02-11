package main

import (
	"fmt"

	"github.com/goradd/goradd/pkg/sys"
	"github.com/goradd/serve/internal/testsite/server"
	_ "github.com/goradd/serve/internal/testsite/web"
	_ "github.com/goradd/serve/internal/testsite/web/form"
)

func main() {
	a := server.NewApplication()

	fmt.Println("\nLaunching server on " + sys.GetIpAddress())
	err := a.RunServer()
	if err != nil {
		fmt.Println(err)
	}
}
