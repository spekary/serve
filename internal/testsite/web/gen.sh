#!/bin/zsh

go run ../../../cmd/serve parse -d ./form ./src/*.html

got -f -v -t tpl.got -i -I github.com/goradd/serve/page/macros.inc.got -d ./form
