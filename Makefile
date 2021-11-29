SRC_FILES=$(shell find . -type f -name '*.go')

all: wp2hugo

wp2hugo: $(SRC_FILES)
	go build -o $@

.PHONY: clean
clean:
	rm -fv wp2hugo
