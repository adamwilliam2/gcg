.PHONY: gen

all: gen

filePath=output
gen:
	GODEBUG=execwait=2 go run main.go gcg -f=$(filePath)