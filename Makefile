.PHONY: gen

all: gen

filePath=output
gen:
	go run main.go gcg -f=$(filePath)