BINARY   := superping
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
DIST     := dist
PKG      := github.com/superping/superping
LDFLAGS  := -s -w -X $(PKG)/internal/appinfo.Version=$(VERSION)
WINFLAGS := $(LDFLAGS) -H=windowsgui
GOFLAGS  := -trimpath
GOBIN    := $(shell go env GOPATH)/bin
WIN_CC   ?= x86_64-w64-mingw32-gcc
WINRES   := rsrc_windows_amd64.syso
WINVER   := $(shell echo $(VERSION) | sed 's/^v//; s/-.*//')

.PHONY: all build build-windows windows-resource test audit run clean icons install-tools deb rpm

all: build

## build: compile the application into $(DIST)
build:
	@mkdir -p $(DIST)
	CGO_ENABLED=1 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY) .
	@echo "built $(DIST)/$(BINARY) $(VERSION)"

## windows-resource: write the icon and the version information into a Windows resource file
windows-resource:
	$(GOBIN)/go-winres simply \
		--arch amd64 \
		--manifest gui \
		--admin \
		--icon assets/icons/icon.png \
		--product-version $(WINVER).0 \
		--file-version $(WINVER).0 \
		--file-description "Portable network monitoring tool with real time ping graphs" \
		--product-name "SuperPinger" \
		--copyright "Copyright (c) Diego Cortassa" \
		--original-filename "$(BINARY).exe" \
		$(WINRES)

## build-windows: cross compile the Windows executable into $(DIST)
build-windows: windows-resource
	@mkdir -p $(DIST)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(WIN_CC) \
		go build $(GOFLAGS) -ldflags '$(WINFLAGS)' -o $(DIST)/$(BINARY).exe .
	@echo "built $(DIST)/$(BINARY).exe $(VERSION)"

## test: run the unit tests
test:
	go test ./...

## audit: run the quality control checks
audit:
	go mod tidy -diff
	go mod verify
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "gofmt found unformatted files" && exit 1)
	go vet ./...
	$(GOBIN)/staticcheck ./...
	$(GOBIN)/govulncheck ./...

## run: start the application from the sources
run:
	go run .

## clean: remove the build artifacts
clean:
	rm -rf $(DIST)

## icons: regenerate the PNG assets from the SVG sources
icons:
	@mkdir -p assets/icons
	convert -background none -density 400 assets/icon.svg -resize 512x512 -depth 8 PNG32:assets/icons/icon.png
	convert -background none -density 400 assets/tray.svg -resize 64x64 -depth 8 PNG32:assets/icons/tray.png
	@echo "regenerated assets/icons/icon.png and assets/icons/tray.png"

## install-tools: install the tools used by the audit target
install-tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/tc-hib/go-winres@latest

## deb: build a Debian package
deb: build
	@mkdir -p $(DIST)/deb/DEBIAN
	@mkdir -p $(DIST)/deb/usr/bin $(DIST)/deb/usr/share/doc/$(BINARY) $(DIST)/deb/usr/share/icons/hicolor/512x512/apps
	cp $(DIST)/$(BINARY) $(DIST)/deb/usr/bin/$(BINARY)
	cp assets/icons/icon.png $(DIST)/deb/usr/share/icons/hicolor/512x512/apps/$(BINARY).png
	cp README.md $(DIST)/deb/usr/share/doc/$(BINARY)/README.md
	@printf 'Package: %s\nVersion: %s\nSection: net\nPriority: optional\nArchitecture: %s\nMaintainer: SuperPinger <superping@example.com>\nDescription: Portable network monitoring tool\n Real time ping monitoring with graphs and daily rotated logs.\n' \
		"$(BINARY)" "$(VERSION)" "$$(dpkg --print-architecture)" > $(DIST)/deb/DEBIAN/control
	dpkg-deb --build $(DIST)/deb $(DIST)/$(BINARY)_$(VERSION)_amd64.deb
	@echo "built $(DIST)/$(BINARY)_$(VERSION)_amd64.deb"

## rpm: build an RPM package
rpm: build
	@mkdir -p $(DIST)/rpm/BUILDROOT/usr/bin $(DIST)/rpm/BUILDROOT/usr/share/icons/hicolor/512x512/apps
	@mkdir -p $(DIST)/rpm/SPECS $(DIST)/rpm/SOURCES $(DIST)/rpm/Build $(DIST)/rpm/RPMS $(DIST)/rpm/SRPMS
	cp $(DIST)/$(BINARY) $(DIST)/rpm/BUILDROOT/usr/bin/$(BINARY)
	cp assets/icons/icon.png $(DIST)/rpm/BUILDROOT/usr/share/icons/hicolor/512x512/apps/$(BINARY).png
	@printf 'Name: %s\nVersion: %s\nRelease: 1\nSummary: Portable network monitoring tool\nLicense: BSD-3-Clause\nBuildArch: x86_64\nRequires: glibc\n%%description\nReal time ping monitoring with graphs and daily rotated logs.\n' \
		"$(BINARY)" "$(VERSION)" > $(DIST)/rpm/SPECS/$(BINARY).spec
	rpmbuild --define "_topdir $(CURDIR)/$(DIST)/rpm" -bb $(DIST)/rpm/SPECS/$(BINARY).spec
	@echo "built $(DIST)/rpm/RPMS/x86_64/$(BINARY)-$(VERSION)-1.x86_64.rpm"
