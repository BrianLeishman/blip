.PHONY: all test firmware flash run probe enclosure
all: test build/blip firmware
build/blip: $(shell find cmd internal dashboard -name '*.go') go.mod go.sum
	mkdir -p build
	go build -o $@ ./cmd/blip
test:
	go test ./...
	go vet ./...
firmware:
	mkdir -p build
	tinygo build -target=feather-rp2040 -size=short -o build/blip.uf2 ./firmware
flash: firmware
	tinygo flash -target=feather-rp2040 ./firmware
run: build/blip
	./build/blip -config blip.local.json
probe: build/blip
	./build/blip -probe
enclosure:
	mkdir -p enclosure/stl
	openscad -o enclosure/stl/front.stl -D 'part="front"' enclosure/blip.scad
	openscad -o enclosure/stl/back.stl -D 'part="back"' enclosure/blip.scad
