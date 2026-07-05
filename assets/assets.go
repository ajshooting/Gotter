package assets

import (
	"embed"
	"io/fs"
)

//go:embed migrations/* static/* templates/*
var files embed.FS

func MustSub(dir string) fs.FS {
	sub, err := fs.Sub(files, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func Migrations() fs.FS {
	return MustSub("migrations")
}

func Static() fs.FS {
	return MustSub("static")
}

func Templates() fs.FS {
	return MustSub("templates")
}
