module ironite/klondike

go 1.24

require (
	golang.org/x/image v0.12.0
	golang.org/x/text v0.13.0
)

replace (
	golang.org/x/image => ../third_party/goimage
	golang.org/x/text => ../third_party/gotext
)
