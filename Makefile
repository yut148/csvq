BINARY=csvq
VERSION=$(shell git describe --tags)

SOURCEDIR=.
SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

LDFLAGS := -ldflags="-X main.version=$(VERSION)"

.DEFAULT_GOAL: $(BINARY)

$(BINARY): $(SOURCES)
	go build $(LDFLAGS) -o $(BINARY)

.PHONY: deps
deps: glide
	glide install

.PHONY: install
install:
	go install $(LDFLAGS)

.PHONY: glide
glide:
ifeq ($(shell command -v glide 2> /dev/null),)
	curl https://glide.sh/get | sh
endif

.PHONY: test
test:
	go test -cover `glide novendor`

.PHONY: clean
clean:
	if [ -f $(BINARY) ]; then rm $(BINARY); fi

.PHONY: release
release:
	git tag $(VERSION)
	git push origin $(VERSION)