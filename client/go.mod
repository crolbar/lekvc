module github.com/crolbar/lekvc/lekvc

go 1.24.4

require (
	github.com/crolbar/lekvc/lekvcs v0.0.0-00010101000000-000000000000
	github.com/gen2brain/malgo v0.11.24
	golang.org/x/sys v0.36.0
)

replace github.com/crolbar/lekvc/lekvcs => ../server
