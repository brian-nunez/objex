module github.com/brian-nunez/objex/examples/filesystem

go 1.22.2

replace github.com/brian-nunez/objex => ../../

replace github.com/brian-nunez/objex/drivers/filesystem => ../../drivers/filesystem

require github.com/brian-nunez/objex/drivers/filesystem v0.0.0

require github.com/brian-nunez/objex v1.0.3 // indirect
