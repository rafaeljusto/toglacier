package config

// Version stores the release version of the tool. It is replaced in build time
// with ldflags argument:
//
//     go build -ldflags "-X github.com/rafaeljusto/toglacier/internal/config.Version=beta"
var Version = "development"
