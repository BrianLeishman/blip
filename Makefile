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
	openscad -o enclosure/stl/bezel-fit-mirrored-short.stl -D 'part="fit"' enclosure/blip.scad

.PHONY: enclosure-slim
enclosure-slim:
	mkdir -p enclosure/stl
	openscad --hardwarnings -o enclosure/stl/slim-front-m25.stl -D 'part="front"' enclosure/slim.scad
	openscad --hardwarnings -o enclosure/stl/slim-stand.stl -D 'part="stand"' enclosure/slim.scad
	openscad --hardwarnings -o enclosure/stl/slim-usb-plate.stl -D 'part="usb-plate"' enclosure/slim.scad
	openscad --hardwarnings -o enclosure/stl/slim-face-fit-m25.stl -D 'part="fit"' enclosure/slim.scad

.PHONY: enclosure-tapered
enclosure-tapered:
	mkdir -p enclosure/stl
	openscad --hardwarnings -o enclosure/stl/tapered-front-m25.stl -D 'part="front"' enclosure/tapered.scad
	openscad --hardwarnings -o enclosure/stl/tapered-stand.stl -D 'part="stand"' enclosure/tapered.scad
	openscad --hardwarnings -o enclosure/stl/tapered-face-fit-m25.stl -D 'part="fit"' enclosure/tapered.scad

.PHONY: enclosure-v2-check
enclosure-v2-check:
	python3 scripts/v2-check.py

.PHONY: enclosure-v2-fit
enclosure-v2-fit:
	mkdir -p enclosure/stl
	openscad --hardwarnings -o enclosure/stl/v2-front-fit-m25.stl enclosure/v2/front-fit.scad
	cp enclosure/stl/v2-front-fit-m25.stl enclosure/stl/v2-front-fit-m25-taller-posts.stl

.PHONY: enclosure-v2
enclosure-v2:
	mkdir -p enclosure/stl
	openscad --hardwarnings -o enclosure/stl/v2-front-m25.stl -D 'part="front"' enclosure/v2/assembly.scad
	openscad --hardwarnings -o enclosure/stl/v2-stand.stl -D 'part="stand"' enclosure/v2/assembly.scad
	openscad --hardwarnings -o enclosure/stl/v2-usb-plate.stl -D 'part="usb-plate"' enclosure/v2/assembly.scad
	openscad --hardwarnings -o enclosure/stl/v2-print-layout.stl -D 'part="print-layout"' enclosure/v2/assembly.scad
	python3 scripts/v2-mesh-check.py
