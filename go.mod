module blazeai

go 1.25.0

toolchain go1.26.4

require (
	github.com/reeflective/readline v1.3.0
	golang.org/x/image v0.43.0
	golang.org/x/term v0.44.0
)

require (
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/sys v0.46.0 // indirect
)

replace github.com/reeflective/readline => ./third_party/readline
