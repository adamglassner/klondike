module ironite/mkres

go 1.24

require (
	github.com/tc-hib/winres v0.0.0
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	golang.org/x/image v0.12.0
	golang.org/x/text v0.13.0
)

replace (
	github.com/tc-hib/winres => ../../third_party/winres
	github.com/nfnt/resize => ../../third_party/nfntresize
	golang.org/x/image => ../../third_party/goimage
	golang.org/x/text => ../../third_party/gotext
)
